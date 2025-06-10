package manager_test

import (
	"context"
	"slices"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinicians"
	cliniciansTest "github.com/tidepool-org/clinic/clinicians/test"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/sites"
	sitesTest "github.com/tidepool-org/clinic/sites/test"
	"github.com/tidepool-org/clinic/store"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/test"
	"github.com/tidepool-org/go-common/clients/shoreline"
)

func Ptr[T any](value T) *T {
	return &value
}

var _ = Describe("Clinics Manager", func() {
	var patientsService patients.Service

	var cfg *config.Config
	var database *mongo.Database
	var patientsCollection *mongo.Collection
	var cliniciansCollection *mongo.Collection
	var clinicsCollection *mongo.Collection
	var mngr manager.Manager

	var DemoPatientId = "demo"

	BeforeEach(func() {
		var err error
		cfg = &config.Config{ClinicDemoPatientUserId: DemoPatientId}
		database = dbTest.GetTestDatabase()
		patientsCollection = database.Collection("patients")
		cliniciansCollection = database.Collection("clinicians")
		clinicsCollection = database.Collection("clinics")

		lifecycle := fxtest.NewLifecycle(GinkgoT())
		lgr := zap.NewNop().Sugar()

		cliniciansRepo, err := clinicians.NewRepository(database, lgr, lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(cliniciansRepo).ToNot(BeNil())

		clinicsRepo, err := clinics.NewRepository(database, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(clinicsRepo).ToNot(BeNil())

		patientsRepo, err := patients.NewRepository(cfg, database, lgr, lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(patientsRepo).ToNot(BeNil())

		patientsService, err = patients.NewService(patientsRepo, clinicsRepo, nil, lgr, database.Client())
		Expect(err).ToNot(HaveOccurred())
		Expect(patientsService).ToNot(BeNil())

		mngr, err = manager.NewManager(manager.Params{
			Clinics:              clinicsRepo,
			CliniciansRepository: cliniciansRepo,
			Config:               cfg,
			DbClient:             database.Client(),
			PatientsRepository:   patientsRepo,
			PatientsService:      patientsService,
			ShareCodeGenerator:   nil,
			UserService:          nil,
		})
		Expect(err).ToNot(HaveOccurred())

		lifecycle.RequireStart()
	})

	Describe("Delete", func() {
		var clinic *clinics.Clinic

		Context("With existing clinic", func() {
			BeforeEach(func() {
				clinic = clinicsTest.RandomClinic()

				res, err := clinicsCollection.InsertOne(context.Background(), clinic)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				clinicId := res.InsertedID.(primitive.ObjectID)
				clinic.Id = &clinicId
			})

			It("deletes the clinic object", func() {
				err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
				Expect(err).ToNot(HaveOccurred())

				selector := bson.M{
					"_id": clinic.Id,
				}

				count, err := clinicsCollection.CountDocuments(context.Background(), selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int64(0)))
			})

			Context("With demo patient and clinician", func() {
				var patient patients.Patient
				var clinician *clinicians.Clinician

				BeforeEach(func() {
					patient = patientsTest.RandomPatient()
					patient.UserId = &DemoPatientId
					patient.ClinicId = clinic.Id

					res, err := patientsCollection.InsertOne(context.Background(), patient)
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())

					clinician = cliniciansTest.RandomClinician()
					clinician.ClinicId = clinic.Id

					res, err = cliniciansCollection.InsertOne(context.Background(), clinician)
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())
				})

				It("deletes the clinic object", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
					Expect(err).ToNot(HaveOccurred())

					selector := bson.M{
						"_id": clinic.Id,
					}

					count, err := clinicsCollection.CountDocuments(context.Background(), selector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(0)))
				})

				It("deletes the patient object", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
					Expect(err).ToNot(HaveOccurred())

					patientSelector := bson.M{
						"userId":   DemoPatientId,
						"clinicId": patient.ClinicId,
					}
					count, err := patientsCollection.CountDocuments(context.Background(), patientSelector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(0)))
				})

				It("deletes the clinician object", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
					Expect(err).ToNot(HaveOccurred())

					clinicianSelector := bson.M{
						"userId":   clinician.UserId,
						"clinicId": clinician.ClinicId,
					}
					count, err := clinicsCollection.CountDocuments(context.Background(), clinicianSelector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(0)))
				})
			})

			Context("With multiple patients and clinician", func() {
				var patient patients.Patient
				var clinician *clinicians.Clinician

				BeforeEach(func() {
					patient = patientsTest.RandomPatient()
					patient.UserId = &DemoPatientId
					patient.ClinicId = clinic.Id

					res, err := patientsCollection.InsertOne(context.Background(), patient)
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())

					secondPatient := patientsTest.RandomPatient()
					secondPatient.ClinicId = clinic.Id
					res, err = patientsCollection.InsertOne(context.Background(), secondPatient)
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())

					clinician = cliniciansTest.RandomClinician()
					clinician.ClinicId = clinic.Id

					res, err = cliniciansCollection.InsertOne(context.Background(), clinician)
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())
				})

				It("returns an error and doesn't delete the clinic object", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
					Expect(err).To(HaveOccurred())

					selector := bson.M{
						"_id": clinic.Id,
					}

					count, err := clinicsCollection.CountDocuments(context.Background(), selector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(1)))
				})

				It("returns an error and doesn't delete patient objects", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
					Expect(err).To(HaveOccurred())

					selector := bson.M{
						"clinicId": clinic.Id,
					}

					count, err := patientsCollection.CountDocuments(context.Background(), selector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(2)))
				})

				It("returns an error and doesn't delete clinician objects", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
					Expect(err).To(HaveOccurred())

					selector := bson.M{
						"clinicId": clinic.Id,
					}

					count, err := cliniciansCollection.CountDocuments(context.Background(), selector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(1)))
				})

			})
		})
	})

	Describe("GetClinicPatientCount", func() {
		var clinic *clinics.Clinic
		var clinicIdString string

		BeforeEach(func() {
			clinic = clinicsTest.RandomClinic()
			res, err := clinicsCollection.InsertOne(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())

			clinic.Id = Ptr(res.InsertedID.(primitive.ObjectID))
			clinicIdString = clinic.Id.Hex()
		})

		When("the clinic has no patients", func() {
			It("returns the correct patient count", func() {
				patientCount, err := mngr.GetClinicPatientCount(context.Background(), clinicIdString)
				Expect(err).ToNot(HaveOccurred())
				Expect(patientCount).ToNot(BeNil())
				Expect(patientCount.PatientCount).To(Equal(0))
			})
		})

		When("a patient is added to the clinic", func() {
			BeforeEach(func() {
				randomPatient := patientsTest.RandomPatient()
				randomPatient.ClinicId = clinic.Id
				randomPatient.Permissions = &patients.Permissions{View: &patients.Permission{}}

				createdPatient, err := patientsService.Create(context.Background(), randomPatient)
				Expect(err).ToNot(HaveOccurred())
				Expect(createdPatient).ToNot(BeNil())
			})

			It("returns the correct patient count", func() {
				patientCount, err := mngr.GetClinicPatientCount(context.Background(), clinicIdString)
				Expect(err).ToNot(HaveOccurred())
				Expect(patientCount).ToNot(BeNil())
				Expect(patientCount.PatientCount).To(Equal(1))
			})

			When("a demo patient is added to the clinic", func() {
				BeforeEach(func() {
					randomPatient := patientsTest.RandomPatient()
					randomPatient.UserId = &DemoPatientId
					randomPatient.ClinicId = clinic.Id
					randomPatient.Permissions = &patients.Permissions{View: &patients.Permission{}}

					createdPatient, err := patientsService.Create(context.Background(), randomPatient)
					Expect(err).ToNot(HaveOccurred())
					Expect(createdPatient).ToNot(BeNil())
				})

				It("returns the correct patient count", func() {
					patientCount, err := mngr.GetClinicPatientCount(context.Background(), clinicIdString)
					Expect(err).ToNot(HaveOccurred())
					Expect(patientCount).ToNot(BeNil())
					Expect(patientCount.PatientCount).To(Equal(1))
				})

				When("aanother patient is added to the clinic", func() {
					BeforeEach(func() {
						randomPatient := patientsTest.RandomPatient()
						randomPatient.ClinicId = clinic.Id
						randomPatient.Permissions = &patients.Permissions{View: &patients.Permission{}}

						createdPatient, err := patientsService.Create(context.Background(), randomPatient)
						Expect(err).ToNot(HaveOccurred())
						Expect(createdPatient).ToNot(BeNil())
					})

					It("returns the correct patient count", func() {
						patientCount, err := mngr.GetClinicPatientCount(context.Background(), clinicIdString)
						Expect(err).ToNot(HaveOccurred())
						Expect(patientCount).ToNot(BeNil())
						Expect(patientCount.PatientCount).To(Equal(2))
					})
				})
			})
		})
	})

	Context("Sites", func() {
		Describe("CreateSite", func() {
			It("works", func() {
				ctx, mngr, th := newCreateSiteTestHelper(GinkgoTB())

				_, err := mngr.CreateSite(ctx, th.Clinic.Id.Hex(), th.Site.Name)
				Expect(err).To(Succeed())
			})
		})

		Describe("DeleteSite", func() {
			It("works", func() {
				ctx, mngr, th := newCreateSiteTestHelper(GinkgoTB())
				siteId := th.Clinic.Sites[0].Id.Hex()
				clinicId := th.Clinic.Id.Hex()

				Expect(mngr.DeleteSite(ctx, clinicId, siteId)).To(Succeed())
			})

			It("removes the site from clinics", func() {
				ctx, mngr, th := newCreateSiteTestHelper(GinkgoTB())
				siteId := th.Clinic.Sites[0].Id.Hex()
				clinicId := th.Clinic.Id.Hex()
				Expect(mngr.DeleteSite(ctx, clinicId, siteId)).To(Succeed())
				clinic, err := th.ClinicsRepo.Get(ctx, clinicId)
				Expect(err).To(Succeed())

				for _, site := range clinic.Sites {
					Expect(site.Id.Hex()).ToNot(Equal(siteId))
				}
			})

			It("removes the site from patients", func() {
				ctx, mngr, th := newCreateSiteTestHelper(GinkgoTB())
				siteId := th.Clinic.Sites[0].Id.Hex()
				clinicId := th.Clinic.Id.Hex()
				Expect(mngr.DeleteSite(ctx, clinicId, siteId)).To(Succeed())
				patients, err := th.PatientsRepo.List(ctx, &patients.Filter{
					ClinicId: &clinicId,
				}, store.DefaultPagination(), nil)
				Expect(err).To(Succeed())

				for _, patient := range patients.Patients {
					Expect(slices.ContainsFunc(patient.Sites, func(s sites.Site) bool {
						return s.Id.Hex() == siteId
					})).To(BeFalse())
				}
			})
		})

		Describe("UpdateSite", func() {
			It("works", func() {
				ctx, mngr, th := newCreateSiteTestHelper(GinkgoTB())
				siteId := th.Clinic.Sites[0].Id.Hex()
				clinicId := th.Clinic.Id.Hex()
				newSite := &sites.Site{Name: "fooberry-jones"}

				_, err := mngr.UpdateSite(ctx, clinicId, siteId, newSite)
				Expect(err).To(Succeed())
			})

			It("updates the clinic's sites", func() {
				ctx, mngr, th := newCreateSiteTestHelper(GinkgoTB())
				siteId := th.Clinic.Sites[0].Id.Hex()
				clinicId := th.Clinic.Id.Hex()
				newSite := &sites.Site{Name: "fooberry-jones"}
				_, err := mngr.UpdateSite(ctx, clinicId, siteId, newSite)
				Expect(err).To(Succeed())
				clinic, err := th.ClinicsRepo.Get(ctx, clinicId)
				Expect(err).To(Succeed())

				Expect(slices.ContainsFunc(clinic.Sites, func(s sites.Site) bool {
					return s.Name == newSite.Name
				})).To(BeTrue())
			})

			It("updates clinic's patients (denormalized)", func() {
				ctx, mngr, th := newCreateSiteTestHelper(GinkgoTB())
				siteId := th.Clinic.Sites[0].Id.Hex()
				clinicId := th.Clinic.Id.Hex()
				newSite := &sites.Site{Name: "fooberry-jones"}
				_, err := mngr.UpdateSite(ctx, clinicId, siteId, newSite)
				Expect(err).To(Succeed())
				patients, err := th.PatientsRepo.List(ctx, &patients.Filter{
					ClinicId: &clinicId,
				}, store.DefaultPagination(), nil)
				Expect(err).To(Succeed())

				for _, patient := range patients.Patients {
					Expect(slices.ContainsFunc(patient.Sites, func(s sites.Site) bool {
						return s.Id.Hex() == siteId
					})).To(BeFalse())
				}
			})
		})

		Describe("GetWithPatientCounts", func() {
			It("enhances clinic sites with the count of patients for each site", func() {
				ctx, mngr, th := newCreateSiteTestHelper(GinkgoTB())

				patient := patientsTest.RandomPatient()
				patient.ClinicId = th.Clinic.Id
				patient.Sites = th.Clinic.Sites
				created, err := th.PatientsRepo.Create(ctx, patient)
				Expect(err).To(Succeed())
				Expect(len(created.Sites)).To(Equal(1))

				clinic, err := mngr.GetWithPatientCounts(ctx, th.Clinic.Id.Hex())
				Expect(err).To(Succeed())
				Expect(len(clinic.Sites)).To(Equal(1))
				Expect(clinic.Sites[0].Patients).To(Equal(1))
			})
		})
	})
})

type createSiteTestHelper struct {
	Clinician    *clinicians.Clinician
	Clinic       *clinics.Clinic
	ClinicsRepo  clinics.Service
	PatientsRepo patients.Service
	Site         *sites.Site
	mngr         manager.Manager
}

func newCreateSiteTestHelper(t testing.TB) (context.Context, manager.Manager, *createSiteTestHelper) {
	t.Helper()
	ctx := context.Background()
	db := dbTest.GetTestDatabase()
	lifecycle := fxtest.NewLifecycle(t)
	lgr := zap.NewNop().Sugar()
	clinicsRepo, err := clinics.NewRepository(db, lgr, lifecycle)
	if err != nil {
		t.Fatalf("failed to create clinics repo: %s", err)
	}
	patientsRepo, err := patients.NewRepository(&config.Config{}, db, lgr, lifecycle)
	if err != nil {
		t.Fatalf("failed to create patients repo: %s", err)
	}
	cliniciansRepo, err := clinicians.NewRepository(db, lgr, lifecycle)
	if err != nil {
		t.Fatalf("failed to create clinicians repo: %s", err)
	}
	patientsService, err := patients.NewService(patientsRepo, clinicsRepo, nil, lgr, db.Client())
	if err != nil {
		t.Fatalf("failed to create patients service: %s", err)
	}

	params := manager.Params{
		Clinics:              clinicsRepo,
		CliniciansRepository: cliniciansRepo,
		Config:               &config.Config{ClinicDemoPatientUserId: "demo"},
		DbClient:             db.Client(),
		PatientsService:      patientsService,
		ShareCodeGenerator:   newMockShareCodeGenerator(),
		UserService:          newMockUserService(),
	}
	mngr, err := manager.NewManager(params)
	if err != nil {
		t.Fatalf("failed to create new clinics manager: %s", err)
	}
	testClinicInput := clinicsTest.RandomClinic()
	testClinicInput.Sites = []sites.Site{}
	testClinician := cliniciansTest.RandomClinician()
	testClinic, err := mngr.CreateClinic(ctx, &manager.CreateClinic{
		Clinic:        *testClinicInput,
		CreatorUserId: *testClinician.UserId,
	})
	if err != nil {
		t.Fatalf("failed to create test clinic: %s", err)
	}
	if testClinic == nil {
		t.Fatalf("failed to create test clinic")
	}
	if testClinic != nil && testClinic.Id != nil {
		testClinician.ClinicId = testClinic.Id
	}

	preCreatedSite := sitesTest.Random()
	_, err = mngr.CreateSite(ctx, testClinic.Id.Hex(), preCreatedSite.Name)
	if err != nil {
		t.Fatalf("failed to create pre-existing clinic site: %s", err)
	}
	testClinic, err = clinicsRepo.Get(ctx, testClinic.Id.Hex())
	if err != nil {
		t.Fatalf("failed to reload clinic (to pick up pre-existing site): %s", err)
	}

	site := sitesTest.Random()
	for site.Name == preCreatedSite.Name {
		site = sitesTest.Random()
	}

	return ctx, mngr, &createSiteTestHelper{
		Clinician:    testClinician,
		Clinic:       testClinic,
		ClinicsRepo:  clinicsRepo,
		PatientsRepo: patientsRepo,
		Site:         &site,
		mngr:         mngr,
	}
}

func (c *createSiteTestHelper) createTestClinicWithoutSites(t testing.TB) (*clinics.Clinic, *clinicians.Clinician) {
	testClinic := clinicsTest.RandomClinic()
	testClinic.Sites = []sites.Site{}
	testClinician := cliniciansTest.RandomClinician()
	_, err := c.mngr.CreateClinic(context.Background(), &manager.CreateClinic{
		Clinic:        *testClinic,
		CreatorUserId: *testClinician.UserId,
	})
	if err != nil {
		t.Fatalf("failed to create test clinic: %s", err)
	}
	return testClinic, testClinician
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
