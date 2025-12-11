package integration_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	cliniciansRepository "github.com/tidepool-org/clinic/clinicians/repository"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	clinicsRepository "github.com/tidepool-org/clinic/clinics/repository"
	clinicsService "github.com/tidepool-org/clinic/clinics/service"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/patients"
	patientsRepository "github.com/tidepool-org/clinic/patients/repository"
	patientsService "github.com/tidepool-org/clinic/patients/service"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/store"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/test"
	"github.com/tidepool-org/go-common/clients/shoreline"
)

// These Sites and Clinic Tags tests are a little different from the other integration
// tests, in that they're testing the integration of the database with the patients and
// clinics services, as opposed to other out-of-repo services, but since they use the real
// database layer, they're not strictly speaking unit tests, and are hopefully a better fit
// here.

var _ = Describe("Patient Sites", func() {
	var th *testHelper

	BeforeEach(func() {
		th = newTestHelper(GinkgoT())
	})

	When("a patient has an assigned site", func() {
		var patient *patients.Patient
		var clinic *clinics.Clinic

		BeforeEach(func() {
			patient, clinic = th.randomPatientInClinic(th.randomClinic())
			Expect(patient.Sites).ToNot(BeNil())
		})

		When("all sites are cleared from the patient", func() {
			It("hasn't any sites any longer", func() {
				// Notice: We're sending an empty list, not nil here!
				patient.Sites = &[]sites.Site{}
				update := patients.PatientUpdate{
					Patient:  *patient,
					ClinicId: clinic.Id.Hex(),
					UserId:   *patient.UserId,
				}
				updated, err := th.patients.Update(th.ctx, update)
				Expect(err).To(Succeed())
				Expect(updated.Sites).ToNot(BeNil())
				Expect(len(*updated.Sites) == 0).To(Equal(true))
			})
		})

		When("sites aren't included in the request", func() {
			It("doesn't modify sites", func() {
				numSites := len(*patient.Sites)
				// Notice: We're sending nil here, not an empty list.
				patient.Sites = nil
				update := patients.PatientUpdate{
					Patient:  *patient,
					ClinicId: clinic.Id.Hex(),
					UserId:   *patient.UserId,
				}
				updated, err := th.patients.Update(th.ctx, update)
				Expect(err).To(Succeed())
				Expect(updated.Sites).ToNot(BeNil())
				Expect(len(*updated.Sites)).To(Equal(numSites))
			})
		})

		When("a site is renamed", func() {
			It("its patients receive the update", func() {
				site := clinic.Sites[0]
				site.Name = "New Name"
				updated, err := th.clinics.UpdateSite(th.ctx, clinic.Id.Hex(),
					site.Id.Hex(), &site)
				Expect(err).To(Succeed())
				Expect(updated.Name == site.Name).To(Equal(true))

				clinicIdHex := clinic.Id.Hex()
				filter := &patients.Filter{
					ClinicId: &clinicIdHex,
				}
				patients, err := th.patients.List(th.ctx, filter, store.DefaultPagination(),
					[]*store.Sort{})
				Expect(err).To(Succeed())
				Expect(len(patients.Patients) > 0).To(Equal(true))
				var found bool
				for _, patient := range patients.Patients {
					if patient.Sites == nil {
						Fail("patient is expected to have > 0 sites, but does not")
					}
					for _, pSite := range *patient.Sites {
						if pSite.Id.Hex() == site.Id.Hex() {
							Expect(pSite.Name).To(Equal(site.Name))
							found = true
						}
					}
					Expect(found).To(Equal(true), "couldn't find the updated site by id")
				}
			})
		})

		When("two sites are merged", func() {
			It("calculates patient counts", func() {
				srcSite := th.getSiteFromClinic(clinic, (*patient.Sites)[0])
				var patient2 *patients.Patient
				patient2, clinic = th.randomPatientInClinic(clinic)
				dstSite := th.getSiteFromClinic(clinic, (*patient2.Sites)[0])
				Expect(srcSite.Patients).To(Equal(1), fmt.Sprintf("srcSite: %+v", srcSite))
				Expect(dstSite.Patients).To(Equal(1), fmt.Sprintf("dstSite: %+v", dstSite))

				merged, err := th.manager.MergeSite(th.ctx, clinic.Id.Hex(),
					srcSite.Id.Hex(), dstSite.Id.Hex())
				Expect(err).To(Succeed())

				Expect(merged).To(Not(BeNil()))
				Expect(srcSite.Patients + dstSite.Patients).To(Equal(1 + 1))
			})
		})
	})
})

var _ = Describe("Patient Tags", func() {
	var th *testHelper

	BeforeEach(func() {
		th = newTestHelper(GinkgoT())
	})

	When("a clinic has tagged patients", func() {
		var clinic *clinics.Clinic

		BeforeEach(func() {
			_, clinic = th.randomPatientInClinic(th.randomClinic())
		})

		It("has tags that include the count of patients", func() {
			clinic, err := th.clinics.Get(th.ctx, clinic.Id.Hex())
			Expect(err).To(Succeed())
			if len(clinic.PatientTags) <= 0 {
				Failf("expected >0 patient tags, got %d", len(clinic.PatientTags))
			}
			tag := clinic.PatientTags[0]
			if tag.Patients != 1 {
				Failf("expected 1 patient, got %d", tag.Patients)
			}
		})

		It("has tags that keep their patient count when converted to a site", func() {
			tag := clinic.PatientTags[0]
			site, err := th.manager.ConvertPatientTagToSite(th.ctx, clinic.Id.Hex(),
				tag.Id.Hex())
			Expect(err).To(Succeed())
			if site == nil {
				Fail("expected site to not be nil")
			}
			if site.Patients == 0 {
				Failf("expected site.Patients to be > 0, got %d", site.Patients)
			}
			if site.Patients != tag.Patients {
				Failf("expected site.Patients == tag.Patients, got %d vs %d",
					site.Patients, tag.Patients)
			}
		})
	})
})

func testLogger() *zap.SugaredLogger {
	enc := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(GinkgoWriter), zapcore.DebugLevel)
	return zap.New(core).Sugar()
}

type testHelper struct {
	patients    patients.Service
	numPatients int
	clinics     clinics.Service
	manager     manager.Manager

	FullGinkgoTInterface
	ctx    context.Context
	logger *zap.SugaredLogger
}

func newTestHelper(t FullGinkgoTInterface) *testHelper {
	logger := testLogger()
	database := dbTest.GetTestDatabase()
	lifecycle := fxtest.NewLifecycle(GinkgoT())

	cfg := &config.Config{ClinicDemoPatientUserId: "demo"}
	patientsRepo, err := patientsRepository.NewRepository(cfg, database, logger,
		lifecycle)
	Expect(err).To(Succeed())

	clinicsRepo, err := clinicsRepository.NewRepository(database, logger, lifecycle)
	Expect(err).To(Succeed())

	clinics, err := clinicsService.NewService(clinicsRepo, patientsRepo, logger)
	Expect(err).To(Succeed())

	patients, err := patientsService.NewService(cfg, patientsRepo, clinics, nil,
		logger, database.Client())
	Expect(err).To(Succeed())

	cliniciansRepo, err := cliniciansRepository.NewRepository(database, logger, lifecycle)
	Expect(err).To(Succeed())

	params := manager.Params{
		ClinicsService:       clinics,
		CliniciansRepository: cliniciansRepo,
		Config:               &config.Config{ClinicDemoPatientUserId: "demo"},
		DbClient:             database.Client(),
		PatientsService:      patients,
		ShareCodeGenerator:   newMockShareCodeGenerator(),
		UserService:          newMockUserService(),
	}
	manager, err := manager.NewManager(params)
	Expect(err).To(Succeed())

	return &testHelper{
		ctx:                  context.Background(),
		FullGinkgoTInterface: t,
		logger:               logger,
		manager:              manager,
		patients:             patients,
		clinics:              clinics,
	}
}

func (th *testHelper) randomClinic() *clinics.Clinic {
	randomClinic := clinicsTest.RandomClinic()
	clinic, err := th.clinics.Create(th.ctx, randomClinic)
	Expect(err).To(Succeed())
	if len(clinic.PatientTags) <= 0 {
		Failf("expected >0 patient tags, got %d", len(clinic.PatientTags))
	}
	if len(clinic.Sites) < 2 {
		Failf("expected >= 2 sites, got %d", len(clinic.Sites))
	}
	if clinic.Id == nil {
		Fail("expected clinic.Id to not be nil")
	}
	return clinic
}

func (th *testHelper) randomPatientInClinic(clinic *clinics.Clinic) (*patients.Patient, *clinics.Clinic) {
	randomPatient := patientsTest.RandomPatient()
	randomPatient.Permissions.Custodian = nil
	randomPatient.ClinicId = clinic.Id
	Expect(randomPatient.UserId).ToNot(BeNil())
	Expect(len(clinic.PatientTags) > 0).To(Equal(true))
	Expect(len(clinic.Sites) > 0).To(Equal(true))
	sitesForPatient := []sites.Site{clinic.Sites[th.numPatients%len(clinic.Sites)]}
	randomPatient.Sites = &sitesForPatient
	randomPatient.ClinicId = clinic.Id
	firstTag := clinic.PatientTags[0]
	randomPatient.Tags = &[]primitive.ObjectID{*firstTag.Id}
	patient, err := th.patients.Create(th.ctx, randomPatient)
	Expect(err).To(Succeed())
	Expect(patient).ToNot(BeNil())
	Expect(patient.Sites).ToNot(BeNil())
	Expect(len(*patient.Sites) > 0).To(Equal(true))
	Expect(clinic.PatientTags[0].Id.Hex()).To(Equal((*patient.Tags)[0].Hex()))

	// reset clinic to get patient counts
	updated, err := th.clinics.Get(th.ctx, clinic.Id.Hex())
	Expect(err).To(Succeed())
	th.numPatients += 1
	return patient, updated
}

func (th *testHelper) getSiteFromClinic(clinic *clinics.Clinic, site sites.Site) sites.Site {
	for _, cSite := range clinic.Sites {
		if cSite.Id.Hex() == site.Id.Hex() {
			return cSite
		}
	}
	Fail("matching site not found")
	return site // this won't happen
}

type mockUserService struct{}

func newMockUserService() *mockUserService {
	return &mockUserService{}
}

func (m *mockUserService) CreateCustodialAccount(ctx context.Context, patient patients.Patient) (*shoreline.UserData, error) {
	panic("not implemented") // TODO: Implement
}

func (m *mockUserService) GetUser(userId string) (*shoreline.UserData, error) {
	return &shoreline.UserData{
		UserID:   userId,
		Username: "test@example.com",
		Emails:   []string{"test@example.com"},
	}, nil
}

func (m *mockUserService) GetUserProfile(ctx context.Context, userId string) (*patients.Profile, error) {
	return &patients.Profile{}, nil
}

func (m *mockUserService) UpdateCustodialAccount(ctx context.Context, patient patients.Patient) error {
	panic("not implemented") // TODO: Implement
}

func (m *mockUserService) PopulatePatientDetailsFromExistingUser(ctx context.Context, patient *patients.Patient) error {
	panic("not implemented") // TODO: Implement
}

type mockShareCodeGenerator struct{}

func newMockShareCodeGenerator() *mockShareCodeGenerator {
	return &mockShareCodeGenerator{}
}

func (m *mockShareCodeGenerator) Generate() string {
	return test.Faker.Lorem().Word()
}

func Failf(msg string, args ...any) {
	Fail(fmt.Sprintf(msg, args...))
}
