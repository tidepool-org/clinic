package integration_test

import (
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/integration/test"
	"net/http"
	"net/http/httptest"
)

var _ = Describe("Migration Test", Ordered, func() {
	var clinic client.Clinic

	Describe("Flag a clinician for migration", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinicians/%s/migrate", test.TestLegacyClinicUserId), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			Expect(json.NewDecoder(rec.Result().Body).Decode(&clinic)).To(Succeed())
			Expect(clinic.CanMigrate).To(PointTo(Equal(false)))
		})
	})

	Describe("Trigger initial migration", func() {
		It("Fails with empty clinic profile", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/migrate", *clinic.Id), "")
			asLegacyClinic(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusUnprocessableEntity))
		})
	})

	Describe("Incomplete profile update", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, fmt.Sprintf("/v1/clinics/%s", *clinic.Id), "./test/migrate_fixtures/01_update_clinic_incomplete.json")
			asLegacyClinic(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			Expect(json.NewDecoder(rec.Result().Body).Decode(&clinic)).To(Succeed())
			Expect(clinic.CanMigrate).To(PointTo(Equal(false)))
		})
	})

	Describe("Trigger initial migration", func() {
		It("Fails with incomplete clinic profile", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/migrate", *clinic.Id), "")
			asLegacyClinic(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusUnprocessableEntity))
		})
	})

	Describe("Complete profile update", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, fmt.Sprintf("/v1/clinics/%s", *clinic.Id), "./test/migrate_fixtures/02_update_clinic_complete.json")
			asLegacyClinic(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			Expect(json.NewDecoder(rec.Result().Body).Decode(&clinic)).To(Succeed())
			Expect(clinic.CanMigrate).To(PointTo(Equal(true)))
		})
	})

	Describe("Trigger initial migration", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/migrate", *clinic.Id), "")
			asLegacyClinic(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

})
