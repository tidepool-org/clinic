package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/fx/fxtest"

	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/patients"
	patientsrepo "github.com/tidepool-org/clinic/patients/repository"
	storetest "github.com/tidepool-org/clinic/store/test"
)

func createClinic() *client.ClinicV1 {
	GinkgoHelper()
	rec := httptest.NewRecorder()
	req := prepareRequest(http.MethodPost, "/v1/clinics",
		"./test/deviceissues_fixtures/create_clinic.json")
	asClinician(req)

	server.ServeHTTP(rec, req)
	Expect(rec.Result()).ToNot(BeNil())
	Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

	body, err := io.ReadAll(rec.Result().Body)
	Expect(err).ToNot(HaveOccurred())
	clinic := &client.ClinicV1{}
	Expect(json.Unmarshal(body, clinic)).To(Succeed())
	Expect(clinic.Id).ToNot(BeNil())

	rec2 := httptest.NewRecorder()
	endpoint := fmt.Sprintf("/v1/clinics/%s/service_accounts", *clinic.Id)
	req2 := prepareRequest(http.MethodPost, endpoint,
		"./test/serviceaccount_fixtures/02_add_service_account.json")
	asServer(req2)

	server.ServeHTTP(rec2, req2)
	Expect(rec2.Result()).ToNot(BeNil())
	Expect(rec2.Result().StatusCode).To(Equal(http.StatusOK))

	return clinic
}

func createPatient(clinicID string) *client.PatientV1 {
	GinkgoHelper()
	endpoint := fmt.Sprintf("/v1/clinics/%s/patients", clinicID)
	rec := httptest.NewRecorder()
	req := prepareRequest(http.MethodPost, endpoint,
		"./test/deviceissues_fixtures/create_patient.json")
	asClinician(req)

	server.ServeHTTP(rec, req)
	Expect(rec.Result()).ToNot(BeNil())
	Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

	created := &client.PatientV1{}
	Expect(json.NewDecoder(rec.Result().Body).Decode(created)).To(Succeed())

	return created
}

func getPatient(clinicID, patientID string) *client.PatientV1 {
	GinkgoHelper()
	endpoint := fmt.Sprintf("/v1/clinics/%s/patients/%s", clinicID, patientID)
	rec := httptest.NewRecorder()
	req := prepareRequestWithBody(http.MethodGet, endpoint, nil)
	asClinician(req)

	server.ServeHTTP(rec, req)
	Expect(rec.Result()).ToNot(BeNil())
	Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

	patient := &client.PatientV1{}

	dup := &bytes.Buffer{}
	r := io.TeeReader(rec.Result().Body, dup)
	Expect(json.NewDecoder(r).Decode(patient)).To(Succeed())

	return patient
}

func parseDatetime(t client.DatetimeV1) time.Time {
	GinkgoHelper()
	out, err := time.Parse(time.RFC3339Nano, t)
	if err != nil {
		Fail(fmt.Sprintf("expected parseable RFC3339 time, got \"%s\"", t))
	}
	return out
}

func updateDeviceIssues() {
	GinkgoHelper()
	rec := httptest.NewRecorder()
	req := prepareRequestWithBody(http.MethodPost, "/v1/device_issues", nil)
	asServer(req)
	server.ServeHTTP(rec, req)
	Expect(rec.Result()).ToNot(BeNil())
	Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
}

func patientsRepo() patients.Repository {
	GinkgoHelper()
	logger := testLogger()
	database := storetest.GetTestDatabase()
	lifecycle := fxtest.NewLifecycle(GinkgoT())

	cfg := &config.Config{ClinicDemoPatientUserId: "demo"}
	repo, err := patientsrepo.NewRepository(cfg, database, logger, lifecycle)
	Expect(err).To(Succeed())
	return repo
}
