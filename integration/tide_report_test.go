package integration_test

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/client"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Pins the TIDE report: category membership predicates, queried-order
// de-duplication across categories, per-category sorting, the noData
// selector variants, tag scoping and the config echo. The report reads
// relative to the current time (the 8 hour staleness rule), so seeded last
// data dates are anchored to time.Now.
var _ = Describe("TIDE Report", Ordered, func() {
	var auth func(*http.Request)
	var clinicId string
	var tagId string
	var cutoff time.Time

	var veryLow, anyLow, veryHigh, anyHigh, drop, lowUse, meeting, dual,
		neverData, staleData, dexcomDisconnected, noTag client.PatientV1

	now := time.Now().UTC().Truncate(time.Second)
	recent := now.Add(-1 * time.Hour)
	staleBeyondCutoff := now.Add(-45 * 24 * time.Hour)
	staleWithinCutoff := now.Add(-10 * time.Hour)

	seedCGM := func(patient client.PatientV1, lastData *time.Time, metrics map[string]interface{}) {
		GinkgoHelper()
		dates := map[string]interface{}{}
		if lastData != nil {
			dates["lastData"] = lastData.Format(time.RFC3339)
			dates["hasLastData"] = true
		}
		seedSummary(*patient.Id, summaryStats("cgm", primitive.NewObjectID().Hex(), dates,
			map[string]interface{}{"30d": cgmPeriod(metrics)}))
	}

	runReport := func(query url.Values, reqAuth func(*http.Request)) client.TideResponseV1 {
		GinkgoHelper()
		endpoint := fmt.Sprintf("/v1/clinics/%s/tide_report?%s", clinicId, query.Encode())
		req := prepareRequest(http.MethodGet, endpoint, "")
		reqAuth(req)
		resp := do(req)
		expectStatus(resp, http.StatusOK)
		return decodeAs[client.TideResponseV1](resp)
	}

	defaultQuery := func() url.Values {
		return url.Values{
			"period":         {"30d"},
			"tags":           {tagId},
			"lastDataCutoff": {cutoff.Format(time.RFC3339)},
		}
	}

	categoryIds := func(report client.TideResponseV1, category string) []string {
		GinkgoHelper()
		patients, ok := report.Results[category]
		Expect(ok).To(BeTrue(), "expected category %s to be present", category)
		ids := make([]string, 0, len(patients))
		for _, p := range patients {
			Expect(p.Patient.Id).ToNot(BeNil())
			ids = append(ids, *p.Patient.Id)
		}
		return ids
	}

	BeforeAll(func() {
		admin := newStubUser()
		auth = asUser(admin.UserID)
		clinicId = *createClinic(auth).Id

		tag := createPatientTag(clinicId, "tide-"+uniqueId(), auth)
		tagId = *tag.Id
		cutoff = now.Add(-30 * 24 * time.Hour)

		tagged := map[string]interface{}{"tags": []string{tagId}}
		newPatient := func(overrides map[string]interface{}) client.PatientV1 {
			return createCustodialPatient(clinicId, auth, overrides)
		}

		veryLow = newPatient(tagged)
		seedCGM(veryLow, &recent, map[string]interface{}{"timeInVeryLowPercent": 0.10})

		anyLow = newPatient(tagged)
		seedCGM(anyLow, &recent, map[string]interface{}{"timeInAnyLowPercent": 0.10})

		veryHigh = newPatient(tagged)
		seedCGM(veryHigh, &recent, map[string]interface{}{"timeInVeryHighPercent": 0.30})

		anyHigh = newPatient(tagged)
		seedCGM(anyHigh, &recent, map[string]interface{}{"timeInAnyHighPercent": 0.40})

		drop = newPatient(tagged)
		seedCGM(drop, &recent, map[string]interface{}{"timeInTargetPercentDelta": -0.30})

		lowUse = newPatient(tagged)
		seedCGM(lowUse, &recent, map[string]interface{}{"timeCGMUsePercent": 0.30})

		meeting = newPatient(tagged)
		seedCGM(meeting, &recent, map[string]interface{}{
			"timeInTargetPercent": 0.90, "timeCGMUsePercent": 0.95,
		})

		// Qualifies for both timeInVeryLowPercent and timeInVeryHighPercent;
		// must be reported only once, in the first queried category.
		dual = newPatient(tagged)
		seedCGM(dual, &recent, map[string]interface{}{
			"timeInVeryLowPercent": 0.20, "timeInVeryHighPercent": 0.50,
		})

		// noData variants: never had data, data older than the cutoff, and a
		// disconnected dexcom data source with data older than 8 hours.
		neverData = newPatient(tagged)

		staleData = newPatient(tagged)
		seedCGM(staleData, &staleBeyondCutoff, map[string]interface{}{})

		dexcomDisconnected = newPatient(tagged)
		seedCGM(dexcomDisconnected, &staleWithinCutoff, map[string]interface{}{
			"timeCGMUsePercent": 0.9, "timeInTargetPercent": 0.5,
		})
		req := prepareRequestWithBody(http.MethodPut,
			fmt.Sprintf("/v1/patients/%s/data_sources", *dexcomDisconnected.Id),
			jsonBody([]map[string]interface{}{{
				"providerName": "dexcom", "state": "pending",
				"dataSourceId": primitive.NewObjectID().Hex(),
			}}))
		asServer(req)
		expectStatus(do(req), http.StatusOK)

		// Not tagged with the report tag; must not appear anywhere.
		noTag = newPatient(nil)
		seedCGM(noTag, &recent, map[string]interface{}{"timeInVeryLowPercent": 0.50})
	})

	It("allows clinic members and rejects non-members", func() {
		endpoint := fmt.Sprintf("/v1/clinics/%s/tide_report?%s", clinicId, defaultQuery().Encode())

		req := prepareRequest(http.MethodGet, endpoint, "")
		auth(req)
		expectStatus(do(req), http.StatusOK)

		outsider := newStubUser()
		req = prepareRequest(http.MethodGet, endpoint, "")
		asUser(outsider.UserID)(req)
		expectStatus(do(req), http.StatusForbidden)
	})

	It("requires a last data cutoff", func() {
		query := defaultQuery()
		query.Del("lastDataCutoff")
		endpoint := fmt.Sprintf("/v1/clinics/%s/tide_report?%s", clinicId, query.Encode())
		req := prepareRequest(http.MethodGet, endpoint, "")
		asServer(req)
		resp := do(req)
		expectStatus(resp, http.StatusBadRequest)
	})

	It("rejects invalid periods", func() {
		query := defaultQuery()
		query.Set("period", "90d")
		endpoint := fmt.Sprintf("/v1/clinics/%s/tide_report?%s", clinicId, query.Encode())
		req := prepareRequest(http.MethodGet, endpoint, "")
		asServer(req)
		resp := do(req)
		expectStatus(resp, http.StatusBadRequest)
	})

	When("requesting the default report", func() {
		var report client.TideResponseV1

		BeforeAll(func() {
			report = runReport(defaultQuery(), asServer)
		})

		It("returns the seven default categories and noData", func() {
			Expect(report.Results).To(HaveLen(8))
			for _, category := range []string{
				"timeInVeryLowPercent", "timeInAnyLowPercent", "timeInVeryHighPercent",
				"timeInAnyHighPercent", "dropInTimeInTargetPercent", "timeCGMUsePercent",
				"meetingTargets", "noData",
			} {
				Expect(report.Results).To(HaveKey(category))
			}
		})

		It("assigns each patient to its first matching category, sorted by severity", func() {
			// dual qualifies for very low and very high; very low is queried
			// first and sorts descending, so dual (0.20) precedes veryLow (0.10).
			Expect(categoryIds(report, "timeInVeryLowPercent")).To(Equal([]string{*dual.Id, *veryLow.Id}))
			Expect(categoryIds(report, "timeInAnyLowPercent")).To(Equal([]string{*anyLow.Id}))
			Expect(categoryIds(report, "timeInVeryHighPercent")).To(Equal([]string{*veryHigh.Id}))
			Expect(categoryIds(report, "timeInAnyHighPercent")).To(Equal([]string{*anyHigh.Id}))
			Expect(categoryIds(report, "dropInTimeInTargetPercent")).To(Equal([]string{*drop.Id}))
			Expect(categoryIds(report, "timeCGMUsePercent")).To(Equal([]string{*lowUse.Id}))
			Expect(categoryIds(report, "meetingTargets")).To(Equal([]string{*meeting.Id}))
		})

		It("returns noData patients sorted by last data date with never-seen first", func() {
			Expect(categoryIds(report, "noData")).To(Equal([]string{
				*neverData.Id, *staleData.Id, *dexcomDisconnected.Id,
			}))
		})

		It("excludes patients without the requested tags", func() {
			for category := range report.Results {
				Expect(categoryIds(report, category)).ToNot(ContainElement(*noTag.Id))
			}
		})

		It("echoes the report configuration", func() {
			Expect(report.Config.SchemaVersion).To(Equal(2))
			Expect(report.Config.Period).To(Equal("30d"))
			Expect(report.Config.ClinicId).To(PointTo(Equal(clinicId)))
			Expect(report.Config.Tags).To(PointTo(ConsistOf(tagId)))
			Expect(report.Config.HighGlucoseThreshold).To(Equal(10.0))
			Expect(report.Config.LowGlucoseThreshold).To(Equal(3.9))
			Expect(report.Config.VeryHighGlucoseThreshold).To(Equal(13.9))
			Expect(report.Config.VeryLowGlucoseThreshold).To(Equal(3.0))
			Expect(report.Config.ExtremeHighGlucoseThreshold).To(PointTo(Equal(19.4)))
			Expect(report.Config.LastDataCutoff.Equal(cutoff)).To(BeTrue())

			Expect(report.Config.Filters.TimeInVeryLowPercent).To(PointTo(Equal(">0.01")))
			Expect(report.Config.Filters.TimeInAnyLowPercent).To(PointTo(Equal(">0.04")))
			Expect(report.Config.Filters.DropInTimeInTargetPercent).To(PointTo(Equal("<-0.15")))
			Expect(report.Config.Filters.TimeInTargetPercent).To(PointTo(Equal("<0.7")))
			Expect(report.Config.Filters.TimeCGMUsePercent).To(PointTo(Equal("<0.7")))
		})

		It("includes patient details and period metrics in results", func() {
			entry := report.Results["timeInVeryLowPercent"][1]
			Expect(entry.Patient.Id).To(PointTo(Equal(*veryLow.Id)))
			Expect(entry.Patient.FullName).To(PointTo(Equal(veryLow.FullName)))
			Expect(entry.Patient.Tags).To(PointTo(ConsistOf(tagId)))
			Expect(entry.TimeInVeryLowPercent).To(HaveValue(BeNumerically("~", 0.10, 1e-9)))
			Expect(entry.LastData).ToNot(BeNil())
			Expect(entry.LastData.Equal(recent)).To(BeTrue())
		})
	})

	When("requesting a subset of categories", func() {
		var report client.TideResponseV1

		BeforeAll(func() {
			query := defaultQuery()
			query.Set("categories", "timeInVeryHighPercent,meetingTargets")
			report = runReport(query, asServer)
		})

		It("returns only the requested categories plus noData", func() {
			Expect(report.Results).To(HaveLen(3))
			Expect(report.Results).To(HaveKey("timeInVeryHighPercent"))
			Expect(report.Results).To(HaveKey("meetingTargets"))
			Expect(report.Results).To(HaveKey("noData"))
		})

		It("de-duplicates according to the queried category order", func() {
			// With timeInVeryLowPercent absent, dual lands in
			// timeInVeryHighPercent instead, sorted descending.
			Expect(categoryIds(report, "timeInVeryHighPercent")).To(Equal([]string{*dual.Id, *veryHigh.Id}))
			Expect(categoryIds(report, "meetingTargets")).To(Equal([]string{*meeting.Id}))
		})
	})

	When("excluding patients without data", func() {
		It("omits the noData category", func() {
			query := defaultQuery()
			query.Set("excludeNoData", "true")
			report := runReport(query, asServer)
			Expect(report.Results).To(HaveLen(7))
			Expect(report.Results).ToNot(HaveKey("noData"))
		})
	})
})
