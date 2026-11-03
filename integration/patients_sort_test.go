package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/client"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/pointer"
)

var _ = Describe("List Patients Sort and Filter Integration Test", Ordered, ContinueOnFailure, func() {
	var clinic client.ClinicV1

	// listPatientsQuery lists the clinic's patients over the 7d period with the additional
	// query parameters in query.
	listPatientsQuery := func(query url.Values) *httptest.ResponseRecorder {
		query.Set("period", "7d")

		rec := httptest.NewRecorder()
		endpoint := fmt.Sprintf("/v1/clinics/%s/patients?%s", *clinic.Id, query.Encode())
		req := prepareRequest(http.MethodGet, endpoint, "")
		asClinician(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		return rec
	}

	listPatients := func(sort, sortType string) *httptest.ResponseRecorder {
		return listPatientsQuery(url.Values{"sort": {sort}, "sortType": {sortType}})
	}

	// patientNames returns the full names of the patients in rec's successful response.
	patientNames := func(rec *httptest.ResponseRecorder) []string {
		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK),
			"response body: %s", rec.Body.String())

		var patients client.PatientsResponseV1
		Expect(json.NewDecoder(rec.Result().Body).Decode(&patients)).To(Succeed())
		Expect(patients.Data).ToNot(BeNil())
		names := []string{}
		for _, patient := range *patients.Data {
			names = append(names, patient.FullName)
		}
		return names
	}

	Describe("Create a clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics",
				"./test/common_fixtures/01_create_clinic.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &clinic)).To(Succeed())
			Expect(clinic.Id).ToNot(BeNil())
		})
	})

	Describe("Create patients with CGM summaries", func() {
		It("Succeeds", func() {
			th := newTestHelper(GinkgoT())
			clinicId, err := primitive.ObjectIDFromHex(*clinic.Id)
			Expect(err).ToNot(HaveOccurred())

			fixtures := []struct {
				name  string
				tir   *float64
				delta *float64
			}{
				{
					name:  "Rising Patient",
					tir:   pointer.FromAny(0.7),
					delta: pointer.FromAny(0.1),
				},
				{
					name:  "Falling Patient",
					tir:   pointer.FromAny(0.5),
					delta: pointer.FromAny(-0.1),
				},
				{name: "No Delta Patient", tir: pointer.FromAny(0.6)},
				{name: "No Summary Patient"},
			}
			for _, f := range fixtures {
				randomPatient := patientsTest.RandomPatient()
				randomPatient.ClinicId = &clinicId
				randomPatient.FullName = pointer.FromAny(f.name)
				randomPatient.Permissions.Custodian = nil
				randomPatient.Tags = nil
				patient, err := th.patients.Create(th.ctx, randomPatient)
				Expect(err).ToNot(HaveOccurred())
				if f.tir == nil {
					continue
				}

				id := primitive.NewObjectID().Hex()
				summary := client.PatientSummaryV1{
					CgmStats: &client.CgmStatsV1{
						Id: &id,
						Periods: client.CgmPeriodsV1{
							"7d": {
								HasTimeInTargetPercent:   true,
								TimeInTargetPercent:      f.tir,
								TimeInTargetPercentDelta: f.delta,
							},
						},
					},
				}
				body, err := json.Marshal(summary)
				Expect(err).ToNot(HaveOccurred())

				rec := httptest.NewRecorder()
				endpoint := fmt.Sprintf("/v1/patients/%s/summary", *patient.UserId)
				req := prepareRequestWithBody(http.MethodPost, endpoint,
					bytes.NewReader(body))
				asServer(req)

				server.ServeHTTP(rec, req)
				Expect(rec.Result()).ToNot(BeNil())
				Expect(rec.Result().StatusCode).To(Equal(http.StatusOK),
					"response body: %s", rec.Body.String())
			}
		})
	})

	DescribeTable("Sorting by timeInTargetPercentDelta succeeds",
		func(sort, sortType string) {
			rec := listPatients(sort, sortType)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK),
				"response body: %s", rec.Body.String())

			var patients client.PatientsResponseV1
			Expect(json.NewDecoder(rec.Result().Body).Decode(&patients)).To(Succeed())
		},
		Entry("cgm descending", "-timeInTargetPercentDelta", "cgm"),
		Entry("cgm ascending", "+timeInTargetPercentDelta", "cgm"),
		Entry("bgm descending", "-timeInTargetPercentDelta", "bgm"),
		Entry("bgm ascending", "+timeInTargetPercentDelta", "bgm"),
	)

	// Like the other summary metrics, patients without timeInTargetPercent sort last in
	// both directions. The pre-sort uses hasTimeInTargetPercent, because there's no
	// hasTimeInTargetPercentDelta, so a patient with timeInTargetPercent but no delta sorts
	// like MongoDB sorts a missing value: first when ascending, last when descending.
	DescribeTable("Sorting by timeInTargetPercentDelta orders patients",
		func(sort string, expected []string) {
			Expect(patientNames(listPatients(sort, "cgm"))).To(Equal(expected))
		},
		Entry("descending", "-timeInTargetPercentDelta", []string{
			"Rising Patient", "Falling Patient", "No Delta Patient", "No Summary Patient",
		}),
		Entry("ascending", "+timeInTargetPercentDelta", []string{
			"No Delta Patient", "Falling Patient", "Rising Patient", "No Summary Patient",
		}),
	)

	// With includeNullDeltas, a delta filter also matches patients whose delta is null but
	// whose timeInTargetPercent is present ("No Delta Patient"), never patients without a
	// summary. Results use the default sort, by full name.
	DescribeTable("Filtering by timeInTargetPercentDelta",
		func(query url.Values, expected []string) {
			Expect(patientNames(listPatientsQuery(query))).To(Equal(expected))
		},
		Entry("excludes null deltas by default",
			url.Values{"cgm.timeInTargetPercentDelta": {"<=0"}},
			[]string{"Falling Patient"},
		),
		Entry("excludes null deltas when includeNullDeltas is false",
			url.Values{
				"cgm.timeInTargetPercentDelta": {"<=0"},
				"includeNullDeltas":            {"false"},
			},
			[]string{"Falling Patient"},
		),
		Entry("includes null deltas with <=",
			url.Values{
				"cgm.timeInTargetPercentDelta": {"<=0"},
				"includeNullDeltas":            {"true"},
			},
			[]string{"Falling Patient", "No Delta Patient"},
		),
		Entry("includes null deltas with >=",
			url.Values{
				"cgm.timeInTargetPercentDelta": {">=0"},
				"includeNullDeltas":            {"true"},
			},
			[]string{"No Delta Patient", "Rising Patient"},
		),
		Entry("has no effect without a delta filter",
			url.Values{"includeNullDeltas": {"true"}},
			[]string{
				"Falling Patient", "No Delta Patient", "No Summary Patient", "Rising Patient",
			},
		),
		Entry("doesn't loosen non-delta filters",
			url.Values{
				"cgm.timeInTargetPercentDelta": {"<=0"},
				"cgm.timeInTargetPercent":      {">=0.55"},
				"includeNullDeltas":            {"true"},
			},
			[]string{"No Delta Patient"},
		),
		Entry("combines with other OR filters",
			url.Values{
				"cgm.timeInTargetPercentDelta": {"<=0"},
				"tags":                         {"_"},
				"sites":                        {"_"},
				"includeNullDeltas":            {"true"},
			},
			[]string{"Falling Patient", "No Delta Patient"},
		),
	)
})
