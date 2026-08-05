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
		Expect(effectiveTime.After(latestDataTime)).To(BeTrue())
		Expect(patientWithIssues.DeviceIssues.StaleData.ProviderId).
			To(Equal(client.Dexcom))
	})

	It("finds patients with expired device connection invitations", func() {
		uploadPatientExpiredDeviceConnectionInvitation(*clinic.Id, patient)
		start := time.Now()
		updateDeviceIssues()

		patientWithIssues := getPatient(*clinic.Id, *patient.Id)
		Expect(len(patientWithIssues.ConnectionRequests.Dexcom) > 0).To(BeTrue())
		effectiveTime := effectiveTimeFromExpiredInvitation(patientWithIssues, "dexcom")
		Expect(effectiveTime.Before(start)).To(BeTrue())
		Expect(patientWithIssues.DeviceIssues).ToNot(BeNil())
		Expect(patientWithIssues.DeviceIssues.ExpiredConnectionInvitation.ProviderId).
			To(Equal(client.Dexcom))
	})

	Context("stale device connection invitations", func() {
		It("finds patients", func() {
			uploadPatientStaleDeviceConnectionInvitation(*clinic.Id, patient)
			start := time.Now()
			updateDeviceIssues()

			patientWithIssues := getPatient(*clinic.Id, *patient.Id)
			Expect(patientWithIssues.DeviceIssues).ToNot(BeNil())
			effectiveTime := effectiveTimeFromStaleInvitationDexcom(patientWithIssues)
			Expect(effectiveTime.Before(start)).To(BeTrue())
			provider := patientWithIssues.DeviceIssues.StaleConnectionInvitation.ProviderId
			Expect(provider).To(Equal(client.Dexcom))
		})

		It("finds patients using only the newest provider connection request", func() {
			uploadMultiplePatientStaleDeviceConnectionInvitation(*clinic.Id, patient)
			start := time.Now()
			updateDeviceIssues()

			patientWithIssues := getPatient(*clinic.Id, *patient.Id)
			Expect(patientWithIssues.DeviceIssues).ToNot(BeNil())
			effectiveTime := effectiveTimeFromStaleInvitationDexcom(patientWithIssues)
			Expect(effectiveTime.Before(start)).To(BeTrue())
			provider := patientWithIssues.DeviceIssues.StaleConnectionInvitation.ProviderId
			Expect(provider).To(Equal(client.Dexcom))
		})
	})

	It("finds patients with disconnected devices", func() {
		uploadPatientDeviceDisconnected(*clinic.Id, patient)
		start := time.Now()
		updateDeviceIssues()

		patientWithIssues := getPatient(*clinic.Id, *patient.Id)
		Expect(patientWithIssues.DeviceIssues).ToNot(BeNil())
		effectiveTime := effectiveTimeFromDisconnected(patientWithIssues, "dexcom")
		Expect(effectiveTime.Before(start)).To(BeTrue())
		provider := patientWithIssues.DeviceIssues.Disconnected.ProviderId
		Expect(provider).To(Equal(client.Dexcom))
	})

	It("removes the disconnected issue when the device reconnects", func() {
		uploadPatientDeviceDisconnected(*clinic.Id, patient)
		updateDeviceIssues()

		patientWithIssues := getPatient(*clinic.Id, *patient.Id)
		Expect(patientWithIssues.DeviceIssues).ToNot(BeNil())
		Expect(patientWithIssues.DeviceIssues.Disconnected.ProviderId).
			To(Equal(client.Dexcom))

		uploadPatientDeviceConnected(*clinic.Id, patient)
		updateDeviceIssues()

		reconnected := getPatient(*clinic.Id, *patient.Id)
		if reconnected.DeviceIssues != nil {
			Expect(reconnected.DeviceIssues.Disconnected).To(BeZero())
		}
	})

	It("finds patients with erroring devices", func() {
		uploadPatientDeviceErroring(*clinic.Id, patient)
		start := time.Now()
		updateDeviceIssues()

		patientWithIssues := getPatient(*clinic.Id, *patient.Id)
		Expect(patientWithIssues.DeviceIssues).ToNot(BeNil())
		effectiveTime := effectiveTimeFromError(patientWithIssues, "dexcom")
		Expect(effectiveTime.Before(start)).To(BeTrue())
		provider := patientWithIssues.DeviceIssues.Erroring.ProviderId
		Expect(provider).To(Equal(client.Dexcom))
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
	sources := []patients.DataSource{}
	pcrs := patients.ProviderConnectionRequests{
		"dexcom": []patients.ConnectionRequest{
			{
				ProviderName:   "dexcom",
				CreatedTime:    now.Add(-32 * 24 * time.Hour),
				ExpirationTime: now.Add(-time.Hour),
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
			UserId:                     patient.Id,
			ProviderConnectionRequests: pcrs,
			DataSources:                &sources,
		},
	}
	_, err = patientsRepo().Update(ctx, update)
	Expect(err).To(Succeed())
	updatedPatient, err := patientsRepo().Update(ctx, update)
	Expect(err).To(Succeed())
	tx := func(i *[]patients.DataSource) []patients.DataSource { return *i }
	Expect(updatedPatient.DataSources).To(Or(BeNil(), WithTransform(tx, HaveLen(0))))
	Expect(updatedPatient.ProviderConnectionRequests["dexcom"]).ToNot(BeNil())
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
				ProviderName:   "dexcom",
				CreatedTime:    created,
				ExpirationTime: time.Now().Add(time.Hour),
			},
		},
	}
	dataSrcs := &[]patients.DataSource{}
	clinicOID, err := primitive.ObjectIDFromHex(clinicID)
	Expect(err).To(Succeed())
	update := patients.PatientUpdate{
		ClinicId: clinicID,
		UserId:   *patient.Id,
		Patient: patients.Patient{
			ClinicId:                   pointer.FromAny(clinicOID),
			UserId:                     patient.Id,
			ProviderConnectionRequests: pcrs,
			DataSources:                dataSrcs,
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

func uploadMultiplePatientStaleDeviceConnectionInvitation(clinicID string,
	patient *client.PatientV1) {

	GinkgoHelper()
	ctx := context.Background()
	now := time.Now()
	created := now.Add(-60 * time.Hour)
	pcrs := patients.ProviderConnectionRequests{
		// We need only one, because they're kept in order
		"dexcom": patients.ConnectionRequests{
			{
				ProviderName:   "dexcom",
				CreatedTime:    created,
				ExpirationTime: time.Now().Add(time.Hour),
			},
		},
	}
	dataSrcs := &[]patients.DataSource{}
	clinicOID, err := primitive.ObjectIDFromHex(clinicID)
	Expect(err).To(Succeed())
	update := patients.PatientUpdate{
		ClinicId: clinicID,
		UserId:   *patient.Id,
		Patient: patients.Patient{
			ClinicId:                   pointer.FromAny(clinicOID),
			UserId:                     patient.Id,
			ProviderConnectionRequests: pcrs,
			DataSources:                dataSrcs,
		},
	}
	_, err = patientsRepo().Update(ctx, update)
	Expect(err).To(Succeed())

	updatedPatient := getPatient(clinicID, *patient.Id)
	Expect(updatedPatient.ConnectionRequests).ToNot(BeNil())
	Expect(len(updatedPatient.ConnectionRequests.Dexcom) > 0).To(BeTrue())
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

// uploadPatientDeviceConnected moves the patient's dexcom data source into the
// connected state, with data recent enough that no staleData issue applies.
func uploadPatientDeviceConnected(clinicID string, patient *client.PatientV1) {
	GinkgoHelper()
	ctx := context.Background()
	now := time.Now()
	sources := &patients.DataSources{
		{
			DataSourceId:   pointer.FromAny(primitive.NewObjectID()),
			ModifiedTime:   pointer.FromAny(now),
			ProviderName:   "dexcom",
			State:          "connected",
			LatestDataTime: pointer.FromAny(now),
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

	Expect(providerId).To(Equal("dexcom"))
	pcrs := patient.ConnectionRequests.Dexcom
	Expect(len(pcrs) == 1).To(BeTrue(), "len(pcrs) should be 1")
	Expect(pcrs[0].ExpirationTime.IsZero()).To(BeFalse(),
		"expiration time should exist: "+fmt.Sprintf("%+v", pcrs[0]))
	return pcrs[0].ExpirationTime
}

func effectiveTimeFromStaleInvitationDexcom(patient *client.PatientV1) time.Time {
	GinkgoHelper()
	for _, req := range patient.ConnectionRequests.Dexcom {
		if req.CreatedTime.IsZero() {
			Fail(fmt.Sprintf("expected created time to not be Zero"))
		}
		return req.CreatedTime
	}
	Fail(fmt.Sprintf("no device connection request found for providerId dexcom"))
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
