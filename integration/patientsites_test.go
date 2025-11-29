package integration_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/tidepool-org/clinic/clinics"
	clinicsRepository "github.com/tidepool-org/clinic/clinics/repository"
	clinicsService "github.com/tidepool-org/clinic/clinics/service"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/patients"
	patientsRepository "github.com/tidepool-org/clinic/patients/repository"
	patientsService "github.com/tidepool-org/clinic/patients/service"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/sites"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

// These Patient Sites tests are a little different from the other integration tests, in
// that they're testing the integration of the database with the patients and clinics
// services, as opposed to other out-of-repo services, but since they use the real database
// layer, they're not strictly speaking unit tests, and are hopefully a better fit here.

var _ = Describe("Patient Sites", func() {

	When("a patient has an assigned site", func() {
		var patient *patients.Patient
		var clinic *clinics.Clinic
		var patientsSvc patients.Service

		BeforeEach(func() {
			ctx := context.Background()
			logger := testLogger()
			database := dbTest.GetTestDatabase()
			lifecycle := fxtest.NewLifecycle(GinkgoT())

			cfg := &config.Config{ClinicDemoPatientUserId: "demo"}
			patientsRepo, err := patientsRepository.NewRepository(cfg, database, logger, lifecycle)
			Expect(err).To(Succeed())

			clinicsRepo, err := clinicsRepository.NewRepository(database, logger, lifecycle)
			Expect(err).To(Succeed())

			clinicsSvc, err := clinicsService.NewService(clinicsRepo)
			Expect(err).To(Succeed())

			randomClinic := clinicsTest.RandomClinic()
			clinic, err = clinicsRepo.Create(ctx, randomClinic)
			Expect(err).To(Succeed())

			patientsSvc, err = patientsService.NewService(patientsRepo, clinicsSvc, nil, logger,
				database.Client())
			Expect(err).To(Succeed())
			randomPatient := patientsTest.RandomPatient()
			randomPatient.Permissions.Custodian = nil
			randomPatient.ClinicId = clinic.Id
			Expect(randomPatient.UserId).ToNot(BeNil())
			Expect(len(clinic.Sites) > 0).To(Equal(true))
			randomPatient.Sites = &clinic.Sites
			patient, err = patientsSvc.Create(ctx, randomPatient)
			Expect(err).To(Succeed())
			Expect(patient).ToNot(BeNil())
			Expect(patient.Sites).ToNot(BeNil())
			Expect(len(*patient.Sites) > 0).To(Equal(true))
		})

		When("all sites are cleared from the patient", func() {
			var updated *patients.Patient

			BeforeEach(func() {
				var err error
				ctx := context.Background()
				// Notice: We're sending an empty list, not nil here!
				patient.Sites = &[]sites.Site{}
				update := patients.PatientUpdate{
					Patient:  *patient,
					ClinicId: clinic.Id.Hex(),
					UserId:   *patient.UserId,
				}
				updated, err = patientsSvc.Update(ctx, update)
				Expect(err).To(Succeed())
			})

			It("hasn't any sites any longer", func() {
				Expect(updated.Sites).ToNot(BeNil())
				Expect(len(*updated.Sites) == 0).To(Equal(true))
			})
		})

		When("sites aren't included in the request", func() {
			var updated *patients.Patient
			var expected int

			BeforeEach(func() {
				var err error
				ctx := context.Background()
				Expect(patient.Sites).ToNot(BeNil())
				expected = len(*patient.Sites)
				// Notice: We're sending nil here, not an empty list.
				patient.Sites = nil
				update := patients.PatientUpdate{
					Patient:  *patient,
					ClinicId: clinic.Id.Hex(),
					UserId:   *patient.UserId,
				}
				updated, err = patientsSvc.Update(ctx, update)
				Expect(err).To(Succeed())
			})

			It("still has sites", func() {
				Expect(updated.Sites).ToNot(BeNil())
				Expect(len(*updated.Sites)).To(Equal(expected))
			})
		})
	})
})

func testLogger() *zap.SugaredLogger {
	enc := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(GinkgoWriter), zapcore.DebugLevel)
	return zap.New(core).Sugar()
}
