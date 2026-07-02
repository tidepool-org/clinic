package integration_test

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

// Smoke tests for the shared integration test infrastructure: the stub user
// registry, dynamic session tokens, and the API-driven builder helpers all
// other endpoint specs rely on.
var _ = Describe("Test Infrastructure", Ordered, func() {
	It("authenticates dynamically registered users", func() {
		user := newStubUser()
		clinic := createClinic(asUser(user.UserID))
		Expect(clinic.Id).ToNot(BeNil())
		Expect(clinic.ShareCode).ToNot(BeNil())

		clinician := getClinician(*clinic.Id, user.UserID)
		Expect(clinician.Roles).To(ContainElement("CLINIC_ADMIN"))
	})

	It("rejects requests from unregistered tokens", func() {
		req := prepareRequest(http.MethodGet, "/v1/clinics/ffffffffffffffffffffffff", "")
		req.Header.Set("x-tidepool-session-token", "not-a-registered-token")
		resp := do(req)
		expectStatus(resp, http.StatusUnauthorized)
	})

	It("creates custodial patients with arbitrary emails", func() {
		admin := newStubUser()
		clinic := createClinic(asUser(admin.UserID))

		patient := createCustodialPatient(*clinic.Id, asUser(admin.UserID), nil)
		Expect(patient.Id).ToNot(BeNil())
		Expect(patient.Permissions).ToNot(BeNil())
		Expect(patient.Permissions.Custodian).ToNot(BeNil())

		fetched := getPatient(*clinic.Id, *patient.Id)
		Expect(fetched.FullName).To(Equal(patient.FullName))
	})

	It("creates patients from existing users with generic profiles", func() {
		admin := newStubUser()
		clinic := createClinic(asUser(admin.UserID))

		// Creating a patient from an existing user is backend-service-only
		// per auth/policy.rego; clinicians get 403.
		patientUser := newStubUser()
		patient := createPatientFromUser(*clinic.Id, patientUser.UserID, asServer, nil)
		Expect(patient.Id).To(PointTo(Equal(patientUser.UserID)))
		Expect(patient.FullName).To(Equal(fmt.Sprintf("User %s", patientUser.UserID)))
		Expect(patient.Email).To(PointTo(Equal(patientUser.Username)))
	})

	It("supports multiple clinician users in the same clinic", func() {
		admin := newStubUser()
		clinic := createClinic(asUser(admin.UserID))

		member := newStubUser()
		clinician := createClinicianDirect(*clinic.Id, member.UserID, "CLINIC_MEMBER")
		Expect(clinician.Roles).To(ConsistOf("CLINIC_MEMBER"))

		response := listPatients(*clinic.Id, nil, asUser(member.UserID))
		Expect(response.Meta).ToNot(BeNil())
	})
})
