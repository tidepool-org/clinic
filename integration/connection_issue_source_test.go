package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/tidepool-org/clinic/api"
	"github.com/tidepool-org/clinic/client"
)

var _ = Describe("Connection Issue Source Integration Test", Ordered, func() {
	var clinic client.ClinicV1
	var patient api.PatientV1

	patientEndpoint := func() string {
		return fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, *patient.Id)
	}

	getPatient := func() api.PatientV1 {
		GinkgoHelper()
		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodGet, patientEndpoint(), "")
		asClinician(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

		fetched := api.PatientV1{}
		Expect(json.NewDecoder(rec.Result().Body).Decode(&fetched)).To(Succeed())
		return fetched
	}

	expectSource := func(source api.ConnectionIssueSourceV1) {
		GinkgoHelper()
		Expect(getPatient().ConnectionIssueSource).To(PointTo(Equal(source)))
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

	Describe("Create a custodial patient with an email", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients", *clinic.Id),
				"./test/common_fixtures/02_create_patient.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			Expect(json.NewDecoder(rec.Result().Body).Decode(&patient)).To(Succeed())
			Expect(patient.Id).To(PointTo(Not(BeEmpty())))
			Expect(patient.Email).To(PointTo(Not(BeEmpty())))
		})

		It("Sets the connection issue source to device non-specific invitation", func() {
			Expect(patient.ConnectionIssueSource).
				To(PointTo(Equal(api.ConnectionIssueSourceDeviceNonSpecificInvitation)))
			expectSource(api.ConnectionIssueSourceDeviceNonSpecificInvitation)
		})
	})

	Describe("Resend the invitation", func() {
		It("Keeps the connection issue source as device non-specific invitation", func() {
			rec := httptest.NewRecorder()
			endpoint := patientEndpoint() + "/invitation_resent"
			req := prepareRequest(http.MethodPost, endpoint, "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))

			expectSource(api.ConnectionIssueSourceDeviceNonSpecificInvitation)
		})
	})

	Describe("Update the patient", func() {
		It("Ignores the connection issue source sent by the client", func() {
			update := getPatient()
			dexcom := api.ConnectionIssueSourceDexcom
			update.ConnectionIssueSource = &dexcom
			body, err := json.Marshal(update)
			Expect(err).ToNot(HaveOccurred())

			rec := httptest.NewRecorder()
			req := prepareRequestWithBody(http.MethodPut, patientEndpoint(),
				bytes.NewReader(body))
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			expectSource(api.ConnectionIssueSourceDeviceNonSpecificInvitation)
		})
	})

	Describe("Request a provider connection", func() {
		It("Sets the connection issue source to the provider", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, patientEndpoint()+"/connect/dexcom", "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))

			expectSource(api.ConnectionIssueSourceDexcom)
		})
	})

	Describe("Resend the invitation after a provider connection request", func() {
		It("Leaves the provider connection issue source unchanged", func() {
			rec := httptest.NewRecorder()
			endpoint := patientEndpoint() + "/invitation_resent"
			req := prepareRequest(http.MethodPost, endpoint, "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))

			expectSource(api.ConnectionIssueSourceDexcom)
		})
	})

	Describe("Connect a data source", func() {
		updateDataSources := func(sources []api.DataSourceV1) {
			GinkgoHelper()
			body, err := json.Marshal(sources)
			Expect(err).ToNot(HaveOccurred())

			rec := httptest.NewRecorder()
			endpoint := fmt.Sprintf("/v1/patients/%s/data_sources", *patient.Id)
			req := prepareRequestWithBody(http.MethodPut, endpoint, bytes.NewReader(body))
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		}

		It("Sets the connection issue source when a source connects", func() {
			updateDataSources([]api.DataSourceV1{{
				ProviderName: "twiist",
				State:        api.Disconnected,
			}})
			expectSource(api.ConnectionIssueSourceDexcom)

			updateDataSources([]api.DataSourceV1{{
				ProviderName: "twiist",
				State:        api.Connected,
			}})
			expectSource(api.ConnectionIssueSourceTwiist)
		})

		It("Leaves the connection issue source alone while still connected", func() {
			updateDataSources([]api.DataSourceV1{{
				ProviderName: "twiist",
				State:        api.Connected,
			}})
			expectSource(api.ConnectionIssueSourceTwiist)
		})
	})
})
