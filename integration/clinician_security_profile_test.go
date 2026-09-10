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

var _ = Describe("Clinician Security Profile Integration Test", Ordered, func() {
	var clinic client.ClinicV1

	googleProvider := client.IdentityproviderV1{Alias: "google-oidc", Name: "Google"}
	endpoint := fmt.Sprintf("/v1/clinicians/%s/securityProfile", test.TestUserId)

	patch := func(fixture string) *http.Response {
		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodPatch, endpoint, fixture)
		asServer(req)

		server.ServeHTTP(rec, req)
		return rec.Result()
	}

	// The PATCH endpoint returns 204 with no body, so the merged state is
	// verified by reading the security profile embedded in the clinician.
	getSecurityProfile := func() *client.ClinicianSecurityProfileV1 {
		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", *clinic.Id, test.TestUserId), "")
		asServer(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

		var clinician client.ClinicianV1
		Expect(json.NewDecoder(rec.Result().Body).Decode(&clinician)).To(Succeed())
		Expect(clinician.SecurityProfile).ToNot(BeNil())
		return clinician.SecurityProfile
	}

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

	Describe("Update Clinician Security Profile", func() {
		It("Is forbidden for regular users", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPatch, endpoint, "./test/securityprofile_fixtures/01_mfa_event.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusForbidden))
		})

		It("Applies an MFA event", func() {
			res := patch("./test/securityprofile_fixtures/01_mfa_event.json")
			Expect(res).ToNot(BeNil())
			Expect(res.StatusCode).To(Equal(http.StatusNoContent))

			profile := getSecurityProfile()
			Expect(profile.MfaEnabled).To(BeTrue())
			Expect(profile.MfaEnabledTime).To(PointTo(Equal(client.DatetimeV1("2026-06-01T10:00:00Z"))))
			Expect(profile.LastLoginTime).To(BeNil())
			Expect(profile.IdentityProviders).To(BeNil())
		})

		It("Merges a last login event without clobbering MFA fields", func() {
			res := patch("./test/securityprofile_fixtures/02_last_login_event.json")
			Expect(res).ToNot(BeNil())
			Expect(res.StatusCode).To(Equal(http.StatusNoContent))

			profile := getSecurityProfile()
			// Field from this event
			Expect(profile.LastLoginTime).To(PointTo(Equal(client.DatetimeV1("2026-06-09T08:30:00Z"))))
			// Fields from the previous event are preserved
			Expect(profile.MfaEnabled).To(BeTrue())
			Expect(profile.MfaEnabledTime).To(PointTo(Equal(client.DatetimeV1("2026-06-01T10:00:00Z"))))
			Expect(profile.IdentityProviders).To(BeNil())
		})

		It("Merges an identity providers event", func() {
			res := patch("./test/securityprofile_fixtures/03_identity_providers_event.json")
			Expect(res).ToNot(BeNil())
			Expect(res.StatusCode).To(Equal(http.StatusNoContent))

			profile := getSecurityProfile()
			Expect(profile.IdentityProviders).To(PointTo(ConsistOf(googleProvider)))
			Expect(profile.MfaEnabled).To(BeTrue())
			Expect(profile.LastLoginTime).To(PointTo(Equal(client.DatetimeV1("2026-06-09T08:30:00Z"))))
		})
	})

	Describe("Get Clinician", func() {
		It("Returns the merged security profile", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", *clinic.Id, test.TestUserId), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var clinician client.ClinicianV1
			Expect(json.NewDecoder(rec.Result().Body).Decode(&clinician)).To(Succeed())
			Expect(clinician.SecurityProfile).ToNot(BeNil())
			Expect(clinician.SecurityProfile.MfaEnabled).To(BeTrue())
			Expect(clinician.SecurityProfile.MfaEnabledTime).To(PointTo(Equal(client.DatetimeV1("2026-06-01T10:00:00Z"))))
			Expect(clinician.SecurityProfile.LastLoginTime).To(PointTo(Equal(client.DatetimeV1("2026-06-09T08:30:00Z"))))
			Expect(clinician.SecurityProfile.IdentityProviders).To(PointTo(ConsistOf(googleProvider)))
		})
	})

	Describe("Disable MFA", func() {
		It("Clears mfaEnabledTime when mfaEnabled is set to false", func() {
			res := patch("./test/securityprofile_fixtures/04_mfa_disabled_event.json")
			Expect(res).ToNot(BeNil())
			Expect(res.StatusCode).To(Equal(http.StatusNoContent))

			profile := getSecurityProfile()
			Expect(profile.MfaEnabled).To(BeFalse())
			Expect(profile.MfaEnabledTime).To(BeNil())
			// Unrelated fields are preserved
			Expect(profile.LastLoginTime).To(PointTo(Equal(client.DatetimeV1("2026-06-09T08:30:00Z"))))
			Expect(profile.IdentityProviders).To(PointTo(ConsistOf(googleProvider)))
		})

		It("Persists the cleared mfaEnabledTime, confirmed via a separate read", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", *clinic.Id, test.TestUserId), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var clinician client.ClinicianV1
			Expect(json.NewDecoder(rec.Result().Body).Decode(&clinician)).To(Succeed())
			Expect(clinician.SecurityProfile).ToNot(BeNil())
			Expect(clinician.SecurityProfile.MfaEnabled).To(BeFalse())
			Expect(clinician.SecurityProfile.MfaEnabledTime).To(BeNil())
		})
	})

	Describe("Field-scoped partial updates", func() {
		It("Does not clobber mfaEnabled when an unrelated event arrives", func() {
			// Re-enable MFA, then send a login-only event. The login event must not
			// reset mfaEnabled/mfaEnabledTime back to their zero values.
			Expect(patch("./test/securityprofile_fixtures/01_mfa_event.json").StatusCode).To(Equal(http.StatusNoContent))

			res := patch("./test/securityprofile_fixtures/06_second_login_event.json")
			Expect(res.StatusCode).To(Equal(http.StatusNoContent))

			profile := getSecurityProfile()
			// Updated by this event
			Expect(profile.LastLoginTime).To(PointTo(Equal(client.DatetimeV1("2026-07-01T12:00:00Z"))))
			// Preserved from prior events, NOT reset by the login-only event
			Expect(profile.MfaEnabled).To(BeTrue())
			Expect(profile.MfaEnabledTime).To(PointTo(Equal(client.DatetimeV1("2026-06-01T10:00:00Z"))))
			Expect(profile.IdentityProviders).To(PointTo(ConsistOf(googleProvider)))
		})

		It("Rejects a malformed datetime with 400", func() {
			res := patch("./test/securityprofile_fixtures/05_invalid_datetime.json")
			Expect(res.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("Update Clinician Security Profile for an unknown clinician", func() {
		It("Returns not found", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPatch, "/v1/clinicians/0000000000/securityProfile", "./test/securityprofile_fixtures/01_mfa_event.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNotFound))
		})
	})
})
