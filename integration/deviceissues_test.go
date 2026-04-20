package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/fx/fxtest"

	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/patients"
	patientsrepo "github.com/tidepool-org/clinic/patients/repository"
	"github.com/tidepool-org/clinic/pointer"
	storetest "github.com/tidepool-org/clinic/store/test"
)

var _ = Describe("UpdateDeviceIssues", func() {
	var clinic *client.ClinicV1
	var patient *client.PatientV1

	BeforeEach(func() {
		clinic = createClinic()
		patient = createPatient(*clinic.Id)
	})

	It("finds patients with stale data", func() {
		uploadPatientStaleData(*clinic.Id, patient)
		updateDeviceIssues()

		patientWithIssues := getPatient(*clinic.Id, *patient.Id)
		Expect(patientWithIssues.DeviceIssues).ToNot(BeNil())
		effectiveTime := effectiveTimeFromStaleData(patientWithIssues)
		latestDataTime := latestDataTimeByProviderId(patientWithIssues, "dexcom")
		if !effectiveTime.After(latestDataTime) {
			Fail(fmt.Sprintf("expected effective time to be after %s, got %s",
				latestDataTime, effectiveTime))
		}
		provider := patientWithIssues.DeviceIssues.StaleData.ProviderId
		if provider != "dexcom" {
			Fail(fmt.Sprintf("expected dexcom, got %q", provider))
		}
	})

	It("finds patients with expired device connection invitations", func() {
		uploadPatientExpiredDeviceConnectionInvitation(*clinic.Id, patient)
		start := time.Now()
		updateDeviceIssues()

		patientWithIssues := getPatient(*clinic.Id, *patient.Id)
		Expect(patientWithIssues.DeviceIssues).ToNot(BeNil())
		expirationTime := effectiveTimeFromExpiredInvitation(patientWithIssues, "dexcom")
		if expirationTime.After(start) {
			Fail(fmt.Sprintf("expected expiration after test start, got %s",
				expirationTime))
		}
		provider := patientWithIssues.DeviceIssues.ExpiredConnectionInvitation.ProviderId
		if provider != "dexcom" {
			Fail(fmt.Sprintf("expected dexcom, got %q", provider))
		}
	})

	It("finds patients with stale device connection invitations", func() {
		uploadPatientStaleDeviceConnectionInvitation(*clinic.Id, patient)
		start := time.Now()
		updateDeviceIssues()

		patientWithIssues := getPatient(*clinic.Id, *patient.Id)
		Expect(patientWithIssues.DeviceIssues).ToNot(BeNil())
		expirationTime := effectiveTimeFromStaleInvitationDexcom(patientWithIssues)
		if expirationTime.After(start) {
			Fail(fmt.Sprintf("expected effective time to be after %s, got %s",
				time.Now(), expirationTime))
		}
		provider := patientWithIssues.DeviceIssues.StaleConnectionInvitation.ProviderId
		if provider != "dexcom" {
			Fail(fmt.Sprintf("expected dexcom, got %q", provider))
		}
	})

	It("finds patients with disconnected devices", func() {
		uploadPatientDeviceDisconnected(*clinic.Id, patient)
		start := time.Now()
		updateDeviceIssues()

		patientWithIssues := getPatient(*clinic.Id, *patient.Id)
		Expect(patientWithIssues.DeviceIssues).ToNot(BeNil())
		expirationTime := effectiveTimeFromDisconnected(patientWithIssues, "dexcom")
		if expirationTime.After(start) {
			Fail(fmt.Sprintf("expected effective time to be after %s, got %s",
				time.Now(), expirationTime))
		}
		provider := patientWithIssues.DeviceIssues.Disconnected.ProviderId
		if provider != "dexcom" {
			Fail(fmt.Sprintf("expected dexcom, got %q", provider))
		}
	})

	It("finds patients with erroring devices", func() {
		uploadPatientDeviceErroring(*clinic.Id, patient)
		start := time.Now()
		updateDeviceIssues()

		patientWithIssues := getPatient(*clinic.Id, *patient.Id)
		Expect(patientWithIssues.DeviceIssues).ToNot(BeNil())
		expirationTime := effectiveTimeFromError(patientWithIssues, "dexcom")
		if expirationTime.After(start) {
			Fail(fmt.Sprintf("expected effective time to be after %s, got %s",
				time.Now(), expirationTime))
		}
		provider := patientWithIssues.DeviceIssues.Erroring.ProviderId
		if provider != "dexcom" {
			Fail(fmt.Sprintf("expected dexcom, got %q", provider))
		}
	})
})

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

func uploadPatientStaleData(clinicID string, patient *client.PatientV1) {
	GinkgoHelper()
	ctx := context.Background()
	now := time.Now()
	sources := &patients.DataSources{
		{
			DataSourceId:   pointer.FromAny(primitive.NewObjectID()),
			ModifiedTime:   pointer.FromAny(now.Add(-time.Hour)),
			ProviderName:   "dexcom",
			State:          "connected",
			LatestDataTime: pointer.FromAny(now.Add(-1000 * time.Hour)),
		},
	}
	err := patientsRepo().UpdatePatientDataSources(ctx, *patient.Id, sources)
	Expect(err).To(Succeed())

	updatedPatient := getPatient(clinicID, *patient.Id)
	if updatedPatient.DataSources == nil || len(*updatedPatient.DataSources) < 1 {
		Fail("expected a data source, got none")
	}
}

func uploadPatientExpiredDeviceConnectionInvitation(clinicID string,
	patient *client.PatientV1) {

	GinkgoHelper()
	ctx := context.Background()
	now := time.Now()
	sources := &patients.DataSources{
		{
			DataSourceId:   pointer.FromAny(primitive.NewObjectID()),
			ModifiedTime:   pointer.FromAny(now.Add(-time.Hour)),
			ProviderName:   "dexcom",
			State:          "pending",
			ExpirationTime: pointer.FromAny(now.Add(-1000 * time.Hour)),
		},
	}
	err := patientsRepo().UpdatePatientDataSources(ctx, *patient.Id, sources)
	Expect(err).To(Succeed())

	updatedPatient := getPatient(clinicID, *patient.Id)
	if updatedPatient.DataSources == nil || len(*updatedPatient.DataSources) < 1 {
		Fail("expected a data source, got none")
	}
}

func uploadPatientStaleDeviceConnectionInvitation(clinicID string,
	patient *client.PatientV1) {

	GinkgoHelper()
	ctx := context.Background()
	now := time.Now()
	created := now.Add(-60 * time.Hour)
	pcrs := patients.ProviderConnectionRequests{
		"dexcom": patients.ConnectionRequests{
			{
				ProviderName: "dexcom",
				CreatedTime:  created,
			},
		},
	}
	clinicOID, err := primitive.ObjectIDFromHex(clinicID)
	Expect(err).To(Succeed())
	update := patients.PatientUpdate{
		ClinicId: clinicID,
		UserId:   *patient.Id,
		Patient: patients.Patient{
			ClinicId:                   pointer.FromAny(clinicOID),
			UserId:                     pointer.FromAny(*patient.Id),
			ProviderConnectionRequests: pcrs,
		},
	}
	_, err = patientsRepo().Update(ctx, update)
	Expect(err).To(Succeed())

	updatedPatient := getPatient(clinicID, *patient.Id)
	if updatedPatient.ConnectionRequests == nil ||
		len(updatedPatient.ConnectionRequests.Dexcom) < 1 {
		Fail("expected a dexcom connection request, got none")
	}
}

func uploadPatientDeviceDisconnected(clinicID string, patient *client.PatientV1) {
	GinkgoHelper()
	ctx := context.Background()
	now := time.Now()
	sources := &patients.DataSources{
		{
			DataSourceId: pointer.FromAny(primitive.NewObjectID()),
			ModifiedTime: pointer.FromAny(now.Add(-time.Hour)),
			ProviderName: "dexcom",
			State:        "disconnected",
		},
	}
	err := patientsRepo().UpdatePatientDataSources(ctx, *patient.Id, sources)
	Expect(err).To(Succeed())

	updatedPatient := getPatient(clinicID, *patient.Id)
	if updatedPatient.DataSources == nil || len(*updatedPatient.DataSources) < 1 {
		Fail("expected a data source, got none")
	}
}

func uploadPatientDeviceErroring(clinicID string, patient *client.PatientV1) {
	GinkgoHelper()
	ctx := context.Background()
	now := time.Now()
	sources := &patients.DataSources{
		{
			DataSourceId: pointer.FromAny(primitive.NewObjectID()),
			ModifiedTime: pointer.FromAny(now.Add(-time.Hour)),
			ProviderName: "dexcom",
			State:        "error",
		},
	}
	err := patientsRepo().UpdatePatientDataSources(ctx, *patient.Id, sources)
	Expect(err).To(Succeed())

	updatedPatient := getPatient(clinicID, *patient.Id)
	if updatedPatient.DataSources == nil || len(*updatedPatient.DataSources) < 1 {
		Fail("expected a data source, got none")
	}
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

func latestDataTimeByProviderId(patient *client.PatientV1, providerId string) time.Time {
	GinkgoHelper()

	for _, dataSource := range *patient.DataSources {
		if dataSource.ProviderName == providerId {
			if dataSource.LatestDataTime == nil {
				Fail(fmt.Sprintf("expected latest data time to not be nil"))
			}
			return parseDatetime(*dataSource.LatestDataTime)
		}
	}
	Fail(fmt.Sprintf("no data source found for providerId %q", providerId))
	return time.Time{}
}

func effectiveTimeFromStaleData(patient *client.PatientV1) time.Time {
	return parseDatetime(patient.DeviceIssues.StaleData.EffectiveTime)
}

func effectiveTimeFromExpiredInvitation(patient *client.PatientV1, providerId string) (
	_ time.Time) {

	GinkgoHelper()

	for _, dataSource := range *patient.DataSources {
		if dataSource.ProviderName == providerId {
			if dataSource.ExpirationTime == nil {
				Fail(fmt.Sprintf("expected expiration time to not be nil"))
			}
			return parseDatetime(*dataSource.ExpirationTime)
		}
	}
	Fail(fmt.Sprintf("no data source found for providerId %q", providerId))
	return time.Time{}
}

func effectiveTimeFromDisconnected(patient *client.PatientV1, providerId string) (
	_ time.Time) {

	GinkgoHelper()

	for _, dataSource := range *patient.DataSources {
		if dataSource.ProviderName == providerId {
			if dataSource.ModifiedTime == nil {
				Fail(fmt.Sprintf("expected modified time to not be nil"))
			}
			return parseDatetime(*dataSource.ModifiedTime)
		}
	}
	Fail(fmt.Sprintf("no data source found for providerId %q", providerId))
	return time.Time{}
}

func effectiveTimeFromError(patient *client.PatientV1, providerId string) (
	_ time.Time) {

	GinkgoHelper()

	for _, dataSource := range *patient.DataSources {
		if dataSource.ProviderName == providerId {
			if dataSource.ModifiedTime == nil {
				Fail(fmt.Sprintf("expected modified time to not be nil"))
			}
			return parseDatetime(*dataSource.ModifiedTime)
		}
	}
	Fail(fmt.Sprintf("no data source found for providerId %q", providerId))
	return time.Time{}
}

func effectiveTimeFromStaleInvitationDexcom(patient *client.PatientV1) time.Time {
	GinkgoHelper()
	for _, req := range patient.ConnectionRequests.Dexcom {
		if req.CreatedTime.IsZero() {
			Fail(fmt.Sprintf("expected created time to not be Zero"))
		}
		return req.CreatedTime
	}
	Fail(fmt.Sprintf("no device connection requirest found for providerId dexcom"))
	return time.Time{}
}
