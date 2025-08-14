package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/integration/test"
)

var _ = Describe("Service Account Integration Test", Ordered, func() {
	var clinic client.ClinicV1

	Describe("Create a clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/common_fixtures/01_create_clinic.json")
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

	Describe("Add Service Account", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/service_accounts", *clinic.Id), "./test/serviceaccount_fixtures/02_add_service_account.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("List Clinicians", func() {
		It("Returns the service account", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians", *clinic.Id), "")
			asServiceAccount(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var clinicians client.CliniciansV1
			Expect(json.NewDecoder(rec.Result().Body).Decode(&clinicians)).To(Succeed())
			Expect(clinicians).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Id": PointTo(Equal(test.TestServiceAccountUserId)),
			})))
		})
	})

	Describe("List Patients", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/patients", *clinic.Id), "")
			asServiceAccount(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

		})
	})

})
