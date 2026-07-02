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

// Pins the ListPatients query surface: search, tag/site filters, summary
// period filters, date range filters, sorting (including null placement) and
// pagination. This is the hardest query to reimplement on Postgres, so the
// semantics asserted here are the contract for the migration.
var _ = Describe("List Patients", Ordered, func() {
	var adminId string
	var auth func(*http.Request)
	var clinicId string
	var fam string
	var baselineTotal int

	var tag1, tag2 client.PatientTagV1
	var site1 client.SiteV1
	var alpha, beta, carol client.PatientV1
	var alphaName, betaName, carolName, deltaName, epsilonName string
	var alphaReviewTime, betaReviewTime time.Time

	var (
		alphaLastData = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
		betaLastData  = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		carolLastData = time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	)

	patientNames := func(r client.PatientsResponseV1) []string {
		GinkgoHelper()
		Expect(r.Data).ToNot(BeNil())
		names := make([]string, 0, len(*r.Data))
		for _, p := range *r.Data {
			names = append(names, p.FullName)
		}
		return names
	}

	list := func(query url.Values) client.PatientsResponseV1 {
		GinkgoHelper()
		return listPatients(clinicId, query, auth)
	}

	// listFam scopes the list to the patients seeded by this spec.
	listFam := func(query url.Values) client.PatientsResponseV1 {
		GinkgoHelper()
		query.Set("search", fam)
		return list(query)
	}

	expectListError := func(query url.Values, code int) {
		GinkgoHelper()
		endpoint := fmt.Sprintf("/v1/clinics/%s/patients?%s", clinicId, query.Encode())
		req := prepareRequest(http.MethodGet, endpoint, "")
		auth(req)
		resp := do(req)
		expectStatus(resp, code)
	}

	BeforeAll(func() {
		admin := newStubUser()
		adminId = admin.UserID
		auth = asUser(adminId)

		clinic := createClinic(auth)
		clinicId = *clinic.Id
		fam = fmt.Sprintf("fam%s", uniqueId())

		baseline := list(url.Values{})
		Expect(baseline.Meta).ToNot(BeNil())
		baselineTotal = *baseline.Meta.TotalCount

		tag1 = createPatientTag(clinicId, "t1-"+fam, auth)
		tag2 = createPatientTag(clinicId, "t2-"+fam, auth)
		site1 = createSite(clinicId, "s1-"+fam, auth)

		alphaName = fmt.Sprintf("alpha %s", fam)
		betaName = fmt.Sprintf("Beta %s", fam)
		carolName = fmt.Sprintf("carol %s", fam)
		deltaName = fmt.Sprintf("Delta %s", fam)
		epsilonName = fmt.Sprintf("epsilon %s", fam)

		alpha = createCustodialPatient(clinicId, auth, map[string]interface{}{
			"fullName": alphaName, "birthDate": "1990-01-01",
		})
		beta = createCustodialPatient(clinicId, auth, map[string]interface{}{
			"fullName": betaName, "birthDate": "1985-05-05",
		})
		carol = createCustodialPatient(clinicId, auth, map[string]interface{}{
			"fullName": carolName, "birthDate": "2000-12-31",
		})
		createCustodialPatient(clinicId, auth, map[string]interface{}{
			"fullName": deltaName, "birthDate": "1970-07-07",
			"tags":  []string{*tag1.Id, *tag2.Id},
			"sites": []map[string]interface{}{{"id": site1.Id, "name": site1.Name}},
		})
		createCustodialPatient(clinicId, auth, map[string]interface{}{
			"fullName": epsilonName, "birthDate": "1999-09-09",
			"tags":           []string{*tag1.Id},
			"glycemicRanges": map[string]interface{}{"type": "preset", "preset": "adaHighRisk"},
		})

		seedSummary(*alpha.Id, summaryStats("cgm", primitive.NewObjectID().Hex(),
			map[string]interface{}{"lastData": alphaLastData.Format(time.RFC3339), "hasLastData": true},
			map[string]interface{}{
				"14d": cgmPeriod(map[string]interface{}{
					"timeInTargetPercent": 0.85, "timeCGMUsePercent": 0.9,
					"totalRecords": 8, "timeInTargetPercentDelta": -0.2,
				}),
				"30d": cgmPeriod(map[string]interface{}{"timeInTargetPercent": 0.85}),
				"7d":  cgmPeriod(map[string]interface{}{"timeInTargetPercent": 0.2}),
			},
		))
		seedSummary(*beta.Id, summaryStats("cgm", primitive.NewObjectID().Hex(),
			map[string]interface{}{"lastData": betaLastData.Format(time.RFC3339), "hasLastData": true},
			map[string]interface{}{
				"14d": cgmPeriod(map[string]interface{}{
					"timeInTargetPercent": 0.3,
					"totalRecords":        3, "timeInTargetPercentDelta": 0.3,
				}),
				"30d": cgmPeriod(map[string]interface{}{"timeInTargetPercent": 0.3}),
				"7d":  cgmPeriod(map[string]interface{}{"timeInTargetPercent": 0.9}),
			},
		))
		seedSummary(*carol.Id, summaryStats("bgm", primitive.NewObjectID().Hex(),
			map[string]interface{}{"lastData": carolLastData.Format(time.RFC3339), "hasLastData": true},
			map[string]interface{}{
				"14d": bgmPeriod(map[string]interface{}{"averageGlucoseMmol": 8.5, "totalRecords": 4}),
			},
		))

		alphaReviews := addReview(clinicId, *alpha.Id, auth)
		Expect(alphaReviews).ToNot(BeEmpty())
		alphaReviewTime = alphaReviews[0].Time

		betaReviews := addReview(clinicId, *beta.Id, auth)
		Expect(betaReviews).ToNot(BeEmpty())
		betaReviewTime = betaReviews[0].Time
		Expect(betaReviewTime.After(alphaReviewTime)).To(BeTrue())
	})

	Describe("counts", func() {
		It("returns matching count and clinic-wide total count", func() {
			response := listFam(url.Values{})
			Expect(response.Meta).ToNot(BeNil())
			Expect(response.Meta.Count).To(PointTo(Equal(5)))
			Expect(response.Meta.TotalCount).To(PointTo(Equal(baselineTotal + 5)))
			Expect(patientNames(response)).To(ConsistOf(alphaName, betaName, carolName, deltaName, epsilonName))
		})
	})

	Describe("search", func() {
		It("matches full names case-insensitively", func() {
			Expect(patientNames(list(url.Values{"search": {"alpha " + fam}}))).To(ConsistOf(alphaName))
			Expect(patientNames(list(url.Values{"search": {"ALPHA " + fam}}))).To(ConsistOf(alphaName))
		})

		It("matches MRN and email", func() {
			Expect(patientNames(list(url.Values{"search": {*alpha.Mrn}}))).To(ConsistOf(alphaName))
			Expect(patientNames(list(url.Values{"search": {*beta.Email}}))).To(ConsistOf(betaName))
		})

		It("matches birth dates as substrings", func() {
			Expect(patientNames(list(url.Values{"search": {"1985-05"}}))).To(ConsistOf(betaName))
		})
	})

	Describe("tag filters", func() {
		It("matches patients with a single tag", func() {
			response := list(url.Values{"tags": {*tag1.Id}})
			Expect(patientNames(response)).To(ConsistOf(deltaName, epsilonName))
		})

		It("requires all provided tags to match", func() {
			// The tags parameter is not exploded: multiple ids are provided
			// as a single comma-separated value.
			response := list(url.Values{"tags": {fmt.Sprintf("%s,%s", *tag1.Id, *tag2.Id)}})
			Expect(patientNames(response)).To(ConsistOf(deltaName))
		})

		It("returns no patients for a valid but unused tag id", func() {
			response := listFam(url.Values{"tags": {primitive.NewObjectID().Hex()}})
			Expect(patientNames(response)).To(BeEmpty())
		})

		It("returns untagged patients when the tag is not an object id", func() {
			response := listFam(url.Values{"tags": {"untagged"}})
			Expect(patientNames(response)).To(ConsistOf(alphaName, betaName, carolName))
		})
	})

	Describe("site filters", func() {
		It("matches patients assigned to a site", func() {
			response := list(url.Values{"sites": {site1.Id}})
			Expect(patientNames(response)).To(ConsistOf(deltaName))
		})

		It("returns patients without sites when the site is not an object id", func() {
			response := listFam(url.Values{"sites": {"none"}})
			Expect(patientNames(response)).To(ConsistOf(alphaName, betaName, carolName, epsilonName))
		})
	})

	Describe("summary field filters", func() {
		It("filters CGM percentages in the default period (14d)", func() {
			Expect(patientNames(list(url.Values{"cgm.timeInTargetPercent": {">0.5"}}))).To(ConsistOf(alphaName))
			Expect(patientNames(list(url.Values{"cgm.timeInTargetPercent": {"<0.5"}}))).To(ConsistOf(betaName))
		})

		It("treats >= and <= boundaries as inclusive", func() {
			Expect(patientNames(list(url.Values{"cgm.timeInTargetPercent": {">=0.85"}}))).To(ConsistOf(alphaName))
			Expect(patientNames(list(url.Values{"cgm.timeInTargetPercent": {"<=0.3"}}))).To(ConsistOf(betaName))
		})

		It("selects different patients when the period changes", func() {
			Expect(patientNames(list(url.Values{
				"period": {"7d"}, "cgm.timeInTargetPercent": {">0.5"},
			}))).To(ConsistOf(betaName))
			Expect(patientNames(list(url.Values{
				"period": {"30d"}, "cgm.timeInTargetPercent": {">0.5"},
			}))).To(ConsistOf(alphaName))
		})

		It("filters delta metrics", func() {
			// alpha's delta is -0.2, beta's is 0.3
			Expect(patientNames(list(url.Values{"cgm.timeInTargetPercentDelta": {"<0.1"}}))).To(ConsistOf(alphaName))
		})

		It("filters BGM metrics", func() {
			Expect(patientNames(list(url.Values{"bgm.averageGlucoseMmol": {">8.0"}}))).To(ConsistOf(carolName))
		})

		It("rejects float filter values outside the handler grammar", func() {
			// The OpenAPI float grammar is permissive, but the handler only
			// accepts a single integer digit, a dot and up to two decimals,
			// so >9.99 is the largest expressible bound and negative bounds
			// are not expressible at all.
			expectListError(url.Values{"cgm.timeInTargetPercent": {">10.5"}}, http.StatusBadRequest)
			expectListError(url.Values{"cgm.timeInTargetPercentDelta": {"<-0.15"}}, http.StatusBadRequest)
		})

		It("rejects filter values without a comparator", func() {
			expectListError(url.Values{"cgm.timeInTargetPercent": {"0.5"}}, http.StatusBadRequest)
		})

		It("rejects integer filters entirely", func() {
			// Integer filter parameters are currently unusable: the OpenAPI
			// grammar requires a plain integer (>5) while the handler grammar
			// requires a decimal (>5.0), so every value is rejected by one of
			// the two validation layers. The Postgres port must decide
			// whether to preserve or fix this.
			expectListError(url.Values{"cgm.totalRecords": {">5"}}, http.StatusBadRequest)
			expectListError(url.Values{"cgm.totalRecords": {">5.0"}}, http.StatusBadRequest)
		})
	})

	Describe("summary date filters", func() {
		It("treats lastDataFrom as inclusive", func() {
			Expect(patientNames(list(url.Values{
				"cgm.lastDataFrom": {betaLastData.Format(time.RFC3339)},
			}))).To(ConsistOf(alphaName, betaName))

			Expect(patientNames(list(url.Values{
				"cgm.lastDataFrom": {betaLastData.Add(time.Second).Format(time.RFC3339)},
			}))).To(ConsistOf(alphaName))
		})

		It("treats lastDataTo as exclusive", func() {
			Expect(patientNames(list(url.Values{
				"cgm.lastDataTo": {alphaLastData.Format(time.RFC3339)},
			}))).To(ConsistOf(betaName))

			Expect(patientNames(list(url.Values{
				"cgm.lastDataTo": {alphaLastData.Add(time.Second).Format(time.RFC3339)},
			}))).To(ConsistOf(alphaName, betaName))
		})

		It("filters BGM last data dates", func() {
			Expect(patientNames(list(url.Values{
				"bgm.lastDataFrom": {carolLastData.Format(time.RFC3339)},
			}))).To(ConsistOf(carolName))
		})
	})

	Describe("last reviewed filter", func() {
		It("includes patients whose most recent review is at or before the given time", func() {
			Expect(patientNames(list(url.Values{
				"lastReviewed": {alphaReviewTime.Format(time.RFC3339Nano)},
			}))).To(ConsistOf(alphaName))

			Expect(patientNames(list(url.Values{
				"lastReviewed": {betaReviewTime.Format(time.RFC3339Nano)},
			}))).To(ConsistOf(alphaName, betaName))
		})
	})

	Describe("glycemic ranges filter", func() {
		It("omits patients with non-standard ranges", func() {
			response := listFam(url.Values{"omitNonStandardRanges": {"true"}})
			Expect(patientNames(response)).To(ConsistOf(alphaName, betaName, carolName, deltaName))
		})
	})

	Describe("sorting", func() {
		It("sorts by full name case-insensitively by default", func() {
			response := listFam(url.Values{})
			Expect(patientNames(response)).To(Equal([]string{alphaName, betaName, carolName, deltaName, epsilonName}))
		})

		It("sorts by birth date", func() {
			response := listFam(url.Values{"sort": {"-birthDate"}, "sortType": {"cgm"}})
			Expect(patientNames(response)).To(Equal([]string{carolName, epsilonName, alphaName, betaName, deltaName}))
		})

		It("sorts by CGM last data date placing patients without data last on descending sorts", func() {
			response := listFam(url.Values{"sort": {"-lastData"}, "sortType": {"cgm"}})
			names := patientNames(response)
			Expect(names[:2]).To(Equal([]string{alphaName, betaName}))
			Expect(names[2:]).To(ConsistOf(carolName, deltaName, epsilonName))
		})

		It("sorts by summary period metrics placing patients without data last", func() {
			// Summary metric sorts are paired with a has<Metric> descending
			// pre-sort, so patients without the metric always sort last
			// regardless of sort direction.
			response := listFam(url.Values{"sort": {"+timeInTargetPercent"}, "sortType": {"cgm"}, "period": {"30d"}})
			names := patientNames(response)
			Expect(names[:2]).To(Equal([]string{betaName, alphaName}))
			Expect(names[2:]).To(ConsistOf(carolName, deltaName, epsilonName))
		})

		It("sorts by BGM metrics placing patients without data last", func() {
			response := listFam(url.Values{"sort": {"+averageGlucoseMmol"}, "sortType": {"bgm"}})
			names := patientNames(response)
			Expect(names[0]).To(Equal(carolName))
		})

		It("sorts by last reviewed time placing unreviewed patients first on ascending sorts", func() {
			// lastReviewed has no has-flag pre-sort, so Mongo's native null
			// ordering applies: missing values first ascending.
			response := listFam(url.Values{"sort": {"+lastReviewed"}, "sortType": {"cgm"}})
			names := patientNames(response)
			Expect(names[:3]).To(ConsistOf(carolName, deltaName, epsilonName))
			Expect(names[3:]).To(Equal([]string{alphaName, betaName}))
		})

		It("rejects sorts without a sort type", func() {
			expectListError(url.Values{"sort": {"+fullName"}}, http.StatusBadRequest)
		})

		It("rejects invalid sort types", func() {
			expectListError(url.Values{"sort": {"+fullName"}, "sortType": {"xyz"}}, http.StatusBadRequest)
		})

		It("rejects invalid periods", func() {
			expectListError(url.Values{"sort": {"+fullName"}, "sortType": {"cgm"}, "period": {"90d"}}, http.StatusBadRequest)
		})

		It("rejects sorts without an order prefix", func() {
			expectListError(url.Values{"sort": {"fullName"}, "sortType": {"cgm"}}, http.StatusBadRequest)
		})

		It("rejects unknown sort attributes", func() {
			expectListError(url.Values{"sort": {"+bogus"}, "sortType": {"cgm"}}, http.StatusBadRequest)
		})
	})

	Describe("pagination", func() {
		It("returns stable disjoint pages with the filtered count", func() {
			var pages [][]string
			for offset := 0; offset < 6; offset += 2 {
				response := listFam(url.Values{
					"sort": {"+fullName"}, "sortType": {"cgm"},
					"limit": {"2"}, "offset": {fmt.Sprintf("%d", offset)},
				})
				Expect(response.Meta.Count).To(PointTo(Equal(5)))
				pages = append(pages, patientNames(response))
			}

			Expect(pages[0]).To(HaveLen(2))
			Expect(pages[1]).To(HaveLen(2))
			Expect(pages[2]).To(HaveLen(1))

			var combined []string
			for _, page := range pages {
				combined = append(combined, page...)
			}
			Expect(combined).To(Equal([]string{alphaName, betaName, carolName, deltaName, epsilonName}))
		})
	})
})
