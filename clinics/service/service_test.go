package service_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	clinicsRepository "github.com/tidepool-org/clinic/clinics/repository"
	clinicsService "github.com/tidepool-org/clinic/clinics/service"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/sites"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

func Ptr[T any](value T) *T {
	return &value
}

var _ = Describe("Clinics", func() {
	var patientsRepoController *gomock.Controller
	var patientsRepo *patientsTest.MockRepository
	var database *mongo.Database
	var service clinics.Service

	BeforeEach(func() {
		var err error
		database = dbTest.GetTestDatabase()
		lgr := zap.NewNop().Sugar()
		lifecycle := fxtest.NewLifecycle(GinkgoT())
		repository, err := clinicsRepository.NewRepository(database, lgr, lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(repository).ToNot(BeNil())
		patientsRepoController = gomock.NewController(GinkgoT())
		patientsRepo = patientsTest.NewMockRepository(patientsRepoController)
		service, err = clinicsService.NewService(repository, patientsRepo, lgr)
		Expect(err).ToNot(HaveOccurred())
		Expect(service).ToNot(BeNil())
		lifecycle.RequireStart()
	})

	AfterEach(func() {
		patientsRepoController.Finish()
	})

	Describe("GetPatientCountSettings", func() {
		It("returns patient count settings by default for a US default tier clinic", func() {
			clinic := clinicsTest.RandomClinic()
			clinic.Country = Ptr(clinics.CountryCodeUS)
			clinic.Tier = clinics.DefaultTier
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCountSettings, err := service.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(Equal(clinics.DefaultPatientCountSettings()))
		})

		It("returns patient count settings by default for a non-US clinic", func() {
			clinic := clinicsTest.RandomClinic()
			clinic.Country = Ptr("CA")
			clinic.Tier = clinics.DefaultTier
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCountSettings, err := service.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(Equal(&clinics.PatientCountSettings{}))
		})

		It("returns patient count settings by default for a US non-default tier clinic", func() {
			clinic := clinicsTest.RandomClinic()
			clinic.Country = Ptr(clinics.CountryCodeUS)
			clinic.Tier = "tier0200"
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCountSettings, err := service.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(Equal(&clinics.PatientCountSettings{}))
		})

		It("returns patient count settings when set for a US default tier clinic", func() {
			expectedPatientCountSettings := &clinics.PatientCountSettings{
				HardLimit: &clinics.PatientCountLimit{
					Plan: 10,
				},
				SoftLimit: &clinics.PatientCountLimit{
					Plan: 5,
				},
			}

			clinic := clinicsTest.RandomClinic()
			clinic.Country = Ptr(clinics.CountryCodeUS)
			clinic.Tier = clinics.DefaultTier
			clinic.PatientCountSettings = expectedPatientCountSettings
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCountSettings, err := service.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(Equal(expectedPatientCountSettings))
		})

		It("returns patient count settings when set for a non-US clinic", func() {
			expectedPatientCountSettings := &clinics.PatientCountSettings{
				HardLimit: &clinics.PatientCountLimit{
					Plan: 10,
				},
				SoftLimit: &clinics.PatientCountLimit{
					Plan: 5,
				},
			}

			clinic := clinicsTest.RandomClinic()
			clinic.Country = Ptr("CA")
			clinic.Tier = clinics.DefaultTier
			clinic.PatientCountSettings = expectedPatientCountSettings
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCountSettings, err := service.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(Equal(&clinics.PatientCountSettings{}))
		})

		It("returns patient count settings when set for a non-US clinic", func() {
			expectedPatientCountSettings := &clinics.PatientCountSettings{
				HardLimit: &clinics.PatientCountLimit{
					Plan: 10,
				},
				SoftLimit: &clinics.PatientCountLimit{
					Plan: 5,
				},
			}

			clinic := clinicsTest.RandomClinic()
			clinic.Country = Ptr(clinics.CountryCodeUS)
			clinic.Tier = "tier0200"
			clinic.PatientCountSettings = expectedPatientCountSettings
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCountSettings, err := service.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(Equal(&clinics.PatientCountSettings{}))
		})
	})

	Describe("UpdatePatientCountSettings", func() {
		It("returns an error if the clinic is not found", func() {
			expectedPatientCountSettings := &clinics.PatientCountSettings{
				HardLimit: &clinics.PatientCountLimit{
					Plan: 10,
				},
				SoftLimit: &clinics.PatientCountLimit{
					Plan: 5,
				},
			}

			err := service.UpdatePatientCountSettings(context.Background(), primitive.NewObjectID().Hex(), expectedPatientCountSettings)
			Expect(err).To(Equal(clinics.ErrNotFound))
		})

		It("updates the patient count settings for a US default tier clinic", func() {
			clinic := clinicsTest.RandomClinic()
			clinic.Country = Ptr(clinics.CountryCodeUS)
			clinic.Tier = clinics.DefaultTier
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			expectedPatientCountSettings := &clinics.PatientCountSettings{
				HardLimit: &clinics.PatientCountLimit{
					Plan: 10,
				},
				SoftLimit: &clinics.PatientCountLimit{
					Plan: 5,
				},
			}

			err = service.UpdatePatientCountSettings(context.Background(), clinic.Id.Hex(), expectedPatientCountSettings)
			Expect(err).ToNot(HaveOccurred())

			patientCountSettings, err := service.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(Equal(expectedPatientCountSettings))
		})

		It("updates the patient count settings for a non-US default tier clinic", func() {
			clinic := clinicsTest.RandomClinic()
			clinic.Country = Ptr("CA")
			clinic.Tier = clinics.DefaultTier
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			expectedPatientCountSettings := &clinics.PatientCountSettings{
				HardLimit: &clinics.PatientCountLimit{
					Plan: 10,
				},
				SoftLimit: &clinics.PatientCountLimit{
					Plan: 5,
				},
			}

			err = service.UpdatePatientCountSettings(context.Background(), clinic.Id.Hex(), expectedPatientCountSettings)
			Expect(err).ToNot(HaveOccurred())

			patientCountSettings, err := service.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(Equal(&clinics.PatientCountSettings{}))
		})

		It("updates the patient count settings for a US no-default tier clinic", func() {
			clinic := clinicsTest.RandomClinic()
			clinic.Country = Ptr(clinics.CountryCodeUS)
			clinic.Tier = "tier0200"
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			expectedPatientCountSettings := &clinics.PatientCountSettings{
				HardLimit: &clinics.PatientCountLimit{
					Plan: 10,
				},
				SoftLimit: &clinics.PatientCountLimit{
					Plan: 5,
				},
			}

			err = service.UpdatePatientCountSettings(context.Background(), clinic.Id.Hex(), expectedPatientCountSettings)
			Expect(err).ToNot(HaveOccurred())

			patientCountSettings, err := service.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(Equal(&clinics.PatientCountSettings{}))
		})
	})

	Describe("GetPatientCount", func() {
		It("returns patient count by default", func() {
			clinic := clinicsTest.RandomClinic()
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCount, err := service.GetPatientCount(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCount).To(Equal(clinics.NewPatientCount()))
		})

		It("returns patient count when set", func() {
			expectedPatientCount := &clinics.PatientCount{
				Plan: 10,
			}

			clinic := clinicsTest.RandomClinic()
			clinic.PatientCount = expectedPatientCount
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCount, err := service.GetPatientCount(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCount).To(Equal(expectedPatientCount))
		})
	})

	Describe("RefreshPatientCount", func() {
		It("returns an error if the patient repository returns an error", func() {
			testErr := fmt.Errorf("test error")
			clinicIdString := primitive.NewObjectID().Hex()
			patientsRepo.
				EXPECT().
				Counts(gomock.Any(), clinicIdString).
				Return(nil, testErr)

			err := service.RefreshPatientCount(context.Background(), clinicIdString)
			Expect(err).To(Equal(testErr))
		})

		It("returns an error if the clinic is not found", func() {
			clinicIdString := primitive.NewObjectID().Hex()
			patientsRepo.
				EXPECT().
				Counts(gomock.Any(), clinicIdString).
				Return(&patients.Counts{}, nil)

			err := service.RefreshPatientCount(context.Background(), clinicIdString)
			Expect(err).To(Equal(clinics.ErrNotFound))
		})

		It("refreshes the patient count", func() {
			clinic := clinicsTest.RandomClinic()
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientsRepo.
				EXPECT().
				Counts(gomock.Any(), clinic.Id.Hex()).
				Return(&patients.Counts{Total: 10, Demo: 1, Plan: 5}, nil)

			err = service.RefreshPatientCount(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("canAddPatientTag", func() {
		It("returns an error when tags exceed the maximum value", func() {
			clinicWithMaxTags := clinics.Clinic{
				PatientTags: genRandomTags(clinics.MaximumPatientTags),
			}

			err := clinics.AssertCanAddPatientTag(clinicWithMaxTags, clinics.PatientTag{})

			Expect(err).To(MatchError(clinics.ErrMaximumPatientTagsExceeded))
		})

		It("returns an error when the tag to be added is a duplicate", func() {
			tagName := "first"
			firstTag := clinics.PatientTag{Name: tagName, Id: ptr(primitive.NewObjectID())}
			clinicWithDupTag := clinics.Clinic{PatientTags: []clinics.PatientTag{firstTag}}

			dupTag := clinics.PatientTag{Name: tagName, Id: ptr(primitive.NewObjectID())}
			err := clinics.AssertCanAddPatientTag(clinicWithDupTag, dupTag)

			Expect(err).To(MatchError(clinics.ErrDuplicatePatientTagName))
		})
	})

	Describe("CreateSite", func() {
		It("creates the site", func() {
			ctx, th := newRepoTestHelper(GinkgoT())
			site := th.newTestSite("Test Site")

			created, err := th.Repo.CreateSite(ctx, th.Clinic.Id.Hex(), site)
			Expect(err).To(Succeed())
			Expect(created.Name).To(Equal(site.Name))
		})

		It("fails when creating the site would exceed sites.MaxSitesPerClinic", func() {
			ctx, th := newRepoTestHelper(GinkgoT())
			for i := len(th.Clinic.Sites); i < sites.MaxSitesPerClinic; i++ {
				th.createTestSite(fmt.Sprintf("Test Site %d", i))
			}
			site := th.newTestSite("Test Site over limit")
			_, err := th.Repo.CreateSite(ctx, th.Clinic.Id.Hex(), site)
			Expect(err).To(MatchError(ContainSubstring("maximum")))
		})

		It("fails when the site's name is a duplicate within the clinic", func() {
			ctx, th := newRepoTestHelper(GinkgoT())
			site := th.newTestSite(th.Site.Name)
			_, err := th.Repo.CreateSite(ctx, th.Clinic.Id.Hex(), site)
			Expect(err).To(MatchError(ContainSubstring("duplicate")))
		})
	})

	Describe("DeleteSite", func() {
		It("deletes the site", func() {
			ctx, th := newRepoTestHelper(GinkgoT())

			Expect(th.Repo.DeleteSite(ctx, th.Clinic.Id.Hex(), th.Site.Id.Hex())).To(Succeed())
		})
	})

	Describe("UpdateSite", func() {
		It("updates the site", func() {
			ctx, th := newRepoTestHelper(GinkgoT())
			updatedSite := th.Site
			updatedSite.Name = "New Name"

			_, err := th.Repo.UpdateSite(ctx, th.Clinic.Id.Hex(), th.Site.Id.Hex(), updatedSite)
			Expect(err).To(Succeed())

			// double check
			clinic, err := th.Repo.Get(ctx, th.Clinic.Id.Hex())
			Expect(err).To(Succeed())
			Expect(len(clinic.Sites)).To(Equal(1))
			Expect(clinic.Sites[0].Name).To(Equal(updatedSite.Name))
		})

		It("succeeds when renaming a site while at max number of sites", func() {
			ctx, th := newRepoTestHelper(GinkgoT())
			for i := len(th.Clinic.Sites); i < sites.MaxSitesPerClinic; i++ {
				th.createTestSite(fmt.Sprintf("Test Site %d", i))
			}
			renamedSite := th.Site
			renamedSite.Name += " (renamed)"
			_, err := th.Repo.UpdateSite(ctx, th.Clinic.Id.Hex(), renamedSite.Id.Hex(), renamedSite)
			Expect(err).To(Succeed())
		})

		It("fails when the site's name is a duplicate within the clinic", func() {
			ctx, th := newRepoTestHelper(GinkgoT())
			secondSite := th.createTestSite(th.Site.Name + " (second)")
			secondSite.Name = th.Site.Name
			_, err := th.Repo.UpdateSite(ctx, th.Clinic.Id.Hex(), secondSite.Id.Hex(), secondSite)
			Expect(err).To(MatchError(ContainSubstring("duplicate")))
		})
	})
})

func genRandomTags(n int) []clinics.PatientTag {
	tags := make([]clinics.PatientTag, n)
	for i := range n {
		tags[i] = clinics.PatientTag{
			Name: fmt.Sprintf("tag%d", i),
		}
	}
	return tags
}

func ptr[A any](a A) *A {
	return &a
}

type repoTestHelper struct {
	Clinic *clinics.Clinic
	Repo   clinics.Repository
	Site   *sites.Site

	t FullGinkgoTInterface
}

func newRepoTestHelper(t FullGinkgoTInterface) (context.Context, *repoTestHelper) {
	db := dbTest.GetTestDatabase()
	lifecycle := fxtest.NewLifecycle(t)
	repo, err := clinicsRepository.NewRepository(db, zap.NewNop().Sugar(), lifecycle)
	if err != nil {
		t.Fatalf("failed to create new clinic repository: %s", err)
	}
	lifecycle.RequireStart()
	th := &repoTestHelper{
		Repo: repo,
		t:    t,
	}
	ctx := context.Background()
	clinic := th.createTestClinic()
	th.Clinic = clinic
	site := th.newTestSite("New York")
	created, err := repo.CreateSite(ctx, clinic.Id.Hex(), site)
	if err != nil {
		t.Fatalf("failed to create initial site: %s", err)
	}
	clinic, err = th.Repo.Get(ctx, clinic.Id.Hex()) // refresh to grab the newly-created site.
	if err != nil {
		t.Fatalf("failed to re-fetch clinic: %s", err)
	}
	th.Clinic = clinic
	th.Site = created
	return ctx, th
}

func (r *repoTestHelper) createTestClinic() *clinics.Clinic {
	clinic := clinics.NewClinicWithDefaults()
	clinic.Name = Ptr("test clinic")
	// NewClinicWithDefaults doesn't set an Id
	clinic.Id = Ptr(primitive.NewObjectID())
	// NewClinicWithDefaults doesn't set share codes
	code := uuid.NewString()
	clinic.ShareCodes = Ptr([]string{code})
	clinic.CanonicalShareCode = &code
	newClinic, err := r.Repo.Create(context.Background(), clinic)
	if err != nil {
		r.t.Fatalf("failed to create new test clinic: %s\n%+v", err, clinic)
	}
	return newClinic
}

func (r *repoTestHelper) newTestSite(name string) *sites.Site {
	return &sites.Site{
		Name: name,
		Id:   primitive.NewObjectID(),
	}
}

// createTestSite creates the new site and returns it, making its Id available.
func (r *repoTestHelper) createTestSite(name string) *sites.Site {
	ctx := context.Background()
	site := r.newTestSite(name)
	created, err := r.Repo.CreateSite(ctx, r.Clinic.Id.Hex(), site)
	if err != nil {
		r.t.Fatalf("failed to create new test site: %s\n%+v", err, site)
	}
	return created
}
