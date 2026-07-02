package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"

	"github.com/TwiN/deepmerge"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/test"
	"github.com/tidepool-org/go-common/clients/shoreline"
)

var uniqueCounter int64

// uniqueId returns a process-wide unique value for generating names, emails
// and MRNs. The database is shared by all specs in the suite, so every spec
// must use unique data.
func uniqueId() string {
	return fmt.Sprintf("%08d", atomic.AddInt64(&uniqueCounter, 1))
}

func do(req *http.Request) *http.Response {
	GinkgoHelper()
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	resp := rec.Result()
	Expect(resp).ToNot(BeNil())
	return resp
}

// expectStatus asserts the response status code and preserves the body so it
// can still be decoded afterwards.
func expectStatus(resp *http.Response, expected int) {
	GinkgoHelper()
	body, err := io.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())
	resp.Body = io.NopCloser(bytes.NewReader(body))
	Expect(resp.StatusCode).To(Equal(expected), "unexpected status code %d, body: %s", resp.StatusCode, string(body))
}

func decodeAs[T any](resp *http.Response) T {
	GinkgoHelper()
	var result T
	body, err := io.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())
	Expect(json.Unmarshal(body, &result)).To(Succeed(), "unable to decode body: %s", string(body))
	return result
}

func jsonBody(v interface{}) io.Reader {
	GinkgoHelper()
	body, err := json.Marshal(v)
	Expect(err).ToNot(HaveOccurred())
	return bytes.NewReader(body)
}

// fixtureWithOverrides loads a JSON fixture and deep-merges the overrides
// into it.
func fixtureWithOverrides(fixturePath string, overrides map[string]interface{}) io.Reader {
	GinkgoHelper()
	base, err := test.LoadFixture(fixturePath)
	Expect(err).ToNot(HaveOccurred())
	ov, err := json.Marshal(overrides)
	Expect(err).ToNot(HaveOccurred())
	merged, err := deepmerge.JSON(base, ov, deepmerge.Config{
		PreventMultipleDefinitionsOfKeysWithPrimitiveValue: false,
	})
	Expect(err).ToNot(HaveOccurred())
	return bytes.NewReader(merged)
}

// applyOverrides merges overrides into a default request body. A nil override
// value removes the key so callers can omit defaulted fields entirely.
func applyOverrides(body map[string]interface{}, overrides map[string]interface{}) map[string]interface{} {
	for k, v := range overrides {
		if v == nil {
			delete(body, k)
		} else {
			body[k] = v
		}
	}
	return body
}

// newStubUser registers a new Tidepool user with the stub user registry so it
// can authenticate requests (see asUser) and be looked up by the service.
func newStubUser() shoreline.UserData {
	userId := stubUsers.NextUserId()
	email := fmt.Sprintf("user+%s@integration.test", userId)
	user := shoreline.UserData{
		UserID:         userId,
		Username:       email,
		Emails:         []string{email},
		PasswordExists: true,
		EmailVerified:  true,
	}
	stubUsers.AddUser(user)
	return user
}

// createClinic creates a clinic through the API. The authenticating user
// becomes the clinic admin.
func createClinic(auth func(*http.Request)) client.ClinicV1 {
	GinkgoHelper()
	overrides := map[string]interface{}{
		"name": fmt.Sprintf("Integration Clinic %s", uniqueId()),
	}
	body := fixtureWithOverrides("./test/common_fixtures/01_create_clinic.json", overrides)
	req := prepareRequestWithBody(http.MethodPost, "/v1/clinics", body)
	auth(req)
	resp := do(req)
	expectStatus(resp, http.StatusOK)
	return decodeAs[client.ClinicV1](resp)
}

func getClinician(clinicId, clinicianId string) client.ClinicianV1 {
	GinkgoHelper()
	req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", clinicId, clinicianId), "")
	asServer(req)
	resp := do(req)
	expectStatus(resp, http.StatusOK)
	return decodeAs[client.ClinicianV1](resp)
}

// createClinicianDirect adds an existing user as a clinician of the clinic
// without going through the invite flow.
func createClinicianDirect(clinicId, userId string, roles ...string) client.ClinicianV1 {
	GinkgoHelper()
	if len(roles) == 0 {
		roles = []string{"CLINIC_MEMBER"}
	}
	body := map[string]interface{}{
		"id":    userId,
		"name":  fmt.Sprintf("Clinician %s", userId),
		"email": fmt.Sprintf("clinician+%s@integration.test", userId),
		"roles": roles,
	}
	req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/clinicians", clinicId), jsonBody(body))
	asServer(req)
	resp := do(req)
	expectStatus(resp, http.StatusOK)
	return getClinician(clinicId, userId)
}

// createCustodialPatient creates a custodial patient account through the API.
// Defaults can be replaced via overrides; a nil override value removes the
// field from the request.
func createCustodialPatient(clinicId string, auth func(*http.Request), overrides map[string]interface{}) client.PatientV1 {
	GinkgoHelper()
	id := uniqueId()
	body := applyOverrides(map[string]interface{}{
		"fullName":  fmt.Sprintf("Patient %s", id),
		"birthDate": "1990-01-01",
		"mrn":       id,
		"email":     fmt.Sprintf("patient+%s@integration.test", id),
	}, overrides)
	req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients", clinicId), jsonBody(body))
	auth(req)
	resp := do(req)
	expectStatus(resp, http.StatusOK)
	return decodeAs[client.PatientV1](resp)
}

// createPatientFromUser adds an existing (stub registered) user as a patient
// of the clinic.
func createPatientFromUser(clinicId, userId string, auth func(*http.Request), overrides map[string]interface{}) client.PatientV1 {
	GinkgoHelper()
	body := applyOverrides(map[string]interface{}{}, overrides)
	req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients/%s", clinicId, userId), jsonBody(body))
	auth(req)
	resp := do(req)
	expectStatus(resp, http.StatusOK)
	return decodeAs[client.PatientV1](resp)
}

func getPatient(clinicId, patientId string) client.PatientV1 {
	GinkgoHelper()
	req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/patients/%s", clinicId, patientId), "")
	asServer(req)
	resp := do(req)
	expectStatus(resp, http.StatusOK)
	return decodeAs[client.PatientV1](resp)
}

// seedSummary updates the patient summary in all clinics through the
// backend-only summary endpoint. This is the only sanctioned way to seed
// summary data for filter and sort tests.
func seedSummary(patientId string, summary map[string]interface{}) {
	GinkgoHelper()
	req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/patients/%s/summary", patientId), jsonBody(summary))
	asServer(req)
	resp := do(req)
	expectStatus(resp, http.StatusOK)
}

func listPatients(clinicId string, query url.Values, auth func(*http.Request)) client.PatientsResponseV1 {
	GinkgoHelper()
	endpoint := fmt.Sprintf("/v1/clinics/%s/patients", clinicId)
	if len(query) > 0 {
		endpoint = fmt.Sprintf("%s?%s", endpoint, query.Encode())
	}
	req := prepareRequest(http.MethodGet, endpoint, "")
	auth(req)
	resp := do(req)
	expectStatus(resp, http.StatusOK)
	return decodeAs[client.PatientsResponseV1](resp)
}
