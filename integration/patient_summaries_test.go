package integration_test

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/client"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Pins the patient summary lifecycle: updates propagate to every clinic the
// patient is a member of, empty updates clear the summary, and deletion by
// summary id removes it everywhere. Summaries are only writable by backend
// services.
var _ = Describe("Patient Summaries", Ordered, func() {
	var auth func(*http.Request)
	var clinicA, clinicB string
	var patientUserId string

	var lastData = time.Date(2026, 6, 20, 8, 30, 0, 0, time.UTC)

	summaryBody := func(cgmId, bgmId string) map[string]interface{} {
		return mergeSummaries(
			summaryStats("cgm", cgmId,
				map[string]interface{}{"lastData": lastData.Format(time.RFC3339), "hasLastData": true},
				map[string]interface{}{
					"14d": cgmPeriod(map[string]interface{}{"timeInTargetPercent": 0.65, "totalRecords": 7}),
				},
			),
			summaryStats("bgm", bgmId,
				map[string]interface{}{},
				map[string]interface{}{
					"14d": bgmPeriod(map[string]interface{}{"averageGlucoseMmol": 6.2}),
				},
			),
		)
	}

	BeforeAll(func() {
		admin := newStubUser()
		auth = asUser(admin.UserID)

		clinicA = *createClinic(auth).Id
		clinicB = *createClinic(auth).Id

		patientUser := newStubUser()
		patientUserId = patientUser.UserID
		createPatientFromUser(clinicA, patientUserId, asServer, map[string]interface{}{"birthDate": "1992-02-02"})
		createPatientFromUser(clinicB, patientUserId, asServer, map[string]interface{}{"birthDate": "1992-02-02"})
	})

	It("rejects summary updates from clinicians", func() {
		req := prepareRequestWithBody(http.MethodPost,
			fmt.Sprintf("/v1/patients/%s/summary", patientUserId),
			jsonBody(summaryBody(primitive.NewObjectID().Hex(), primitive.NewObjectID().Hex())))
		auth(req)
		resp := do(req)
		expectStatus(resp, http.StatusForbidden)
	})

	When("a summary is updated by a backend service", func() {
		var cgmId, bgmId string

		BeforeAll(func() {
			cgmId = primitive.NewObjectID().Hex()
			bgmId = primitive.NewObjectID().Hex()
			seedSummary(patientUserId, summaryBody(cgmId, bgmId))
		})

		It("is visible in every clinic the patient is a member of", func() {
			for _, clinicId := range []string{clinicA, clinicB} {
				patient := getPatient(clinicId, patientUserId)
				Expect(patient.Summary).ToNot(BeNil(), "expected summary in clinic %s", clinicId)
				Expect(patient.Summary.CgmStats).ToNot(BeNil())
				Expect(patient.Summary.CgmStats.Id).To(HaveValue(Equal(client.SummaryIdV1(cgmId))))
				Expect(patient.Summary.BgmStats).ToNot(BeNil())
				Expect(patient.Summary.BgmStats.Id).To(HaveValue(Equal(client.SummaryIdV1(bgmId))))
			}
		})

		It("round-trips dates and period metrics", func() {
			patient := getPatient(clinicA, patientUserId)
			cgm := patient.Summary.CgmStats
			Expect(cgm.Dates.LastData).ToNot(BeNil())
			Expect(cgm.Dates.LastData.Equal(lastData)).To(BeTrue())
			Expect(cgm.Dates.HasLastData).To(BeTrue())

			period, ok := cgm.Periods["14d"]
			Expect(ok).To(BeTrue())
			Expect(period.TimeInTargetPercent).To(HaveValue(BeNumerically("~", 0.65, 1e-9)))
			Expect(period.TotalRecords).To(HaveValue(Equal(7)))

			bgmPeriod, ok := patient.Summary.BgmStats.Periods["14d"]
			Expect(ok).To(BeTrue())
			Expect(bgmPeriod.AverageGlucoseMmol).To(HaveValue(BeNumerically("~", 6.2, 1e-9)))
		})
	})

	When("a summary is updated with an empty body", func() {
		BeforeAll(func() {
			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/patients/%s/summary", patientUserId), nil)
			asServer(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
		})

		It("clears the summary in every clinic", func() {
			for _, clinicId := range []string{clinicA, clinicB} {
				patient := getPatient(clinicId, patientUserId)
				Expect(patient.Summary).To(BeNil(), "expected summary to be cleared in clinic %s", clinicId)
			}
		})
	})

	When("a summary is deleted by id", func() {
		var cgmId, bgmId string

		BeforeAll(func() {
			cgmId = primitive.NewObjectID().Hex()
			bgmId = primitive.NewObjectID().Hex()
			seedSummary(patientUserId, summaryBody(cgmId, bgmId))

			req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/summaries/%s/clinics", cgmId), "")
			asServer(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
		})

		It("removes the matching summary type in every clinic", func() {
			for _, clinicId := range []string{clinicA, clinicB} {
				patient := getPatient(clinicId, patientUserId)
				if patient.Summary == nil {
					continue
				}
				Expect(patient.Summary.CgmStats).To(BeNil(), "expected cgm stats to be deleted in clinic %s", clinicId)
			}
		})

		It("rejects deletions from clinicians", func() {
			req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/summaries/%s/clinics", bgmId), "")
			auth(req)
			resp := do(req)
			expectStatus(resp, http.StatusForbidden)
		})
	})
})
