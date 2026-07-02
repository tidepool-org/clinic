package integration_test

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/client"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Pins the upload reminder and cross-clinic data source update endpoints.
var _ = Describe("Patients Misc", Ordered, func() {
	var auth func(*http.Request)
	var clinicA, clinicB string

	BeforeAll(func() {
		admin := newStubUser()
		auth = asUser(admin.UserID)
		clinicA = *createClinic(auth).Id
		clinicB = *createClinic(auth).Id
	})

	Describe("upload reminders", func() {
		var patientId string

		BeforeAll(func() {
			patient := createCustodialPatient(clinicA, auth, nil)
			patientId = *patient.Id
		})

		It("records the reminder time", func() {
			req := prepareRequest(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients/%s/upload_reminder", clinicA, patientId), "")
			auth(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)

			updated := decodeAs[client.PatientV1](resp)
			Expect(updated.LastUploadReminderTime).ToNot(BeNil())

			fetched := getPatient(clinicA, patientId)
			Expect(fetched.LastUploadReminderTime).ToNot(BeNil())
			Expect(fetched.LastUploadReminderTime.Equal(*updated.LastUploadReminderTime)).To(BeTrue())
		})

		It("rejects server tokens", func() {
			// The authorization policy only allows clinicians, so server
			// tokens are rejected before the handler's own token check.
			req := prepareRequest(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients/%s/upload_reminder", clinicA, patientId), "")
			asServer(req)
			expectStatus(do(req), http.StatusForbidden)
		})
	})

	Describe("data sources", func() {
		var patientUserId string

		BeforeAll(func() {
			user := newStubUser()
			patientUserId = user.UserID
			createPatientFromUser(clinicA, patientUserId, asServer, nil)
			createPatientFromUser(clinicB, patientUserId, asServer, nil)
		})

		updateDataSources := func(state string) {
			GinkgoHelper()
			req := prepareRequestWithBody(http.MethodPut,
				fmt.Sprintf("/v1/patients/%s/data_sources", patientUserId),
				jsonBody([]map[string]interface{}{{
					"providerName": "dexcom", "state": state,
					"dataSourceId": primitive.NewObjectID().Hex(),
				}}))
			asServer(req)
			expectStatus(do(req), http.StatusOK)
		}

		expectStateInBothClinics := func(state string) {
			GinkgoHelper()
			for _, clinicId := range []string{clinicA, clinicB} {
				patient := getPatient(clinicId, patientUserId)
				Expect(patient.DataSources).ToNot(BeNil())
				Expect(*patient.DataSources).To(HaveLen(1))
				Expect((*patient.DataSources)[0].ProviderName).To(Equal("dexcom"))
				Expect(string((*patient.DataSources)[0].State)).To(Equal(state))
			}
		}

		It("is forbidden for regular users", func() {
			req := prepareRequestWithBody(http.MethodPut,
				fmt.Sprintf("/v1/patients/%s/data_sources", patientUserId),
				jsonBody([]map[string]interface{}{}))
			auth(req)
			expectStatus(do(req), http.StatusForbidden)
		})

		It("updates data sources in every clinic the patient is a member of", func() {
			updateDataSources("pending")
			expectStateInBothClinics("pending")
		})

		It("reflects state transitions", func() {
			updateDataSources("connected")
			expectStateInBothClinics("connected")
		})
	})
})
