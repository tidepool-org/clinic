package integration_test

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson"
)

// Pins patient permission semantics: full-replace updates, single permission
// deletion, the custodian stripping rule, the policy matrix, and the cascade
// that removes a patient from a clinic when the last permission is revoked.
var _ = Describe("Patient Permissions", Ordered, func() {
	var auth func(*http.Request)
	var clinicA, clinicB string
	var patientUserId string

	permissionsOf := func(clinicId string) client.PatientPermissionsV1 {
		GinkgoHelper()
		patient := getPatient(clinicId, patientUserId)
		Expect(patient.Permissions).ToNot(BeNil())
		return *patient.Permissions
	}

	putPermissions := func(clinicId string, body map[string]interface{}, reqAuth func(*http.Request)) *http.Response {
		GinkgoHelper()
		req := prepareRequestWithBody(http.MethodPut,
			fmt.Sprintf("/v1/clinics/%s/patients/%s/permissions", clinicId, patientUserId), jsonBody(body))
		reqAuth(req)
		return do(req)
	}

	deletePermission := func(clinicId, permission string, reqAuth func(*http.Request)) *http.Response {
		GinkgoHelper()
		req := prepareRequest(http.MethodDelete,
			fmt.Sprintf("/v1/clinics/%s/patients/%s/permissions/%s", clinicId, patientUserId, permission), "")
		reqAuth(req)
		return do(req)
	}

	BeforeAll(func() {
		admin := newStubUser()
		auth = asUser(admin.UserID)
		clinicA = *createClinic(auth).Id
		clinicB = *createClinic(auth).Id

		user := newStubUser()
		patientUserId = user.UserID
		createPatientFromUser(clinicA, patientUserId, asServer, map[string]interface{}{
			"permissions": map[string]interface{}{"view": map[string]interface{}{}, "upload": map[string]interface{}{}, "note": map[string]interface{}{}},
		})
		createPatientFromUser(clinicB, patientUserId, asServer, map[string]interface{}{
			"permissions": map[string]interface{}{"view": map[string]interface{}{}},
		})
	})

	It("stores the permissions provided at creation", func() {
		permissions := permissionsOf(clinicA)
		Expect(permissions.View).ToNot(BeNil())
		Expect(permissions.Upload).ToNot(BeNil())
		Expect(permissions.Note).ToNot(BeNil())
		Expect(permissions.Custodian).To(BeNil())
	})

	It("is forbidden for users other than the patient", func() {
		expectStatus(putPermissions(clinicA, map[string]interface{}{"view": map[string]interface{}{}}, auth), http.StatusForbidden)

		other := newStubUser()
		expectStatus(deletePermission(clinicA, "view", asUser(other.UserID)), http.StatusForbidden)
	})

	It("allows backend services to delete only the custodian permission", func() {
		expectStatus(deletePermission(clinicA, "view", asServer), http.StatusForbidden)
	})

	It("replaces the entire permission set on update", func() {
		resp := putPermissions(clinicA, map[string]interface{}{"view": map[string]interface{}{}, "upload": map[string]interface{}{}}, asUser(patientUserId))
		expectStatus(resp, http.StatusOK)

		permissions := permissionsOf(clinicA)
		Expect(permissions.View).ToNot(BeNil())
		Expect(permissions.Upload).ToNot(BeNil())
		Expect(permissions.Note).To(BeNil())
	})

	It("strips custodian permissions from updates", func() {
		// Custodian permission cannot be granted after an account is claimed.
		resp := putPermissions(clinicA, map[string]interface{}{"view": map[string]interface{}{}, "custodian": map[string]interface{}{}}, asUser(patientUserId))
		expectStatus(resp, http.StatusOK)

		permissions := permissionsOf(clinicA)
		Expect(permissions.View).ToNot(BeNil())
		Expect(permissions.Custodian).To(BeNil())
		Expect(permissions.Upload).To(BeNil())
	})

	It("deletes a single permission", func() {
		resp := putPermissions(clinicA, map[string]interface{}{"view": map[string]interface{}{}, "upload": map[string]interface{}{}}, asUser(patientUserId))
		expectStatus(resp, http.StatusOK)

		expectStatus(deletePermission(clinicA, "upload", asUser(patientUserId)), http.StatusNoContent)

		permissions := permissionsOf(clinicA)
		Expect(permissions.View).ToNot(BeNil())
		Expect(permissions.Upload).To(BeNil())
	})

	It("removes the patient from the clinic when the last permission is deleted", func() {
		expectStatus(deletePermission(clinicA, "view", asUser(patientUserId)), http.StatusNoContent)

		req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/patients/%s", clinicA, patientUserId), "")
		asServer(req)
		expectStatus(do(req), http.StatusNotFound)

		// The membership in the other clinic is untouched.
		Expect(permissionsOf(clinicB).View).ToNot(BeNil())
	})

	It("creates a deletion audit record for the cascade removal", func() {
		// PORT-TO-PG: deletion audit records have no read endpoint.
		err := test.GetTestDatabase().Collection("patient_deletions").
			FindOne(testCtx(), bson.M{"patient.userId": patientUserId}).Err()
		Expect(err).ToNot(HaveOccurred())
	})

	It("removes the patient from the clinic when permissions are replaced with an empty set", func() {
		resp := putPermissions(clinicB, map[string]interface{}{}, asUser(patientUserId))
		expectStatus(resp, http.StatusNoContent)

		req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/patients/%s", clinicB, patientUserId), "")
		asServer(req)
		expectStatus(do(req), http.StatusNotFound)
	})

	It("allows backend services to revoke custodian access of custodial accounts", func() {
		custodial := createCustodialPatient(clinicA, auth, nil)
		patientUserId := *custodial.Id

		req := prepareRequest(http.MethodDelete,
			fmt.Sprintf("/v1/clinics/%s/patients/%s/permissions/custodian", clinicA, patientUserId), "")
		asServer(req)
		expectStatus(do(req), http.StatusNoContent)

		patient := getPatient(clinicA, patientUserId)
		Expect(patient.Permissions.Custodian).To(BeNil())
		Expect(patient.Permissions.View).ToNot(BeNil())
	})
})
