package clinics_test

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/sites"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

func Ptr[T any](value T) *T {
	return &value
}

var _ = Describe("Clinics", func() {
	var database *mongo.Database
	var repo clinics.Service

	BeforeEach(func() {
		var err error
		database = dbTest.GetTestDatabase()
		lifecycle := fxtest.NewLifecycle(GinkgoT())
		repo, err = clinics.NewRepository(database, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(repo).ToNot(BeNil())
		lifecycle.RequireStart()
	})

	Describe("GetPatientCountSettings", func() {
		It("returns patient count settings by default", func() {
			clinic := clinicsTest.RandomClinic()
			clinic, err := repo.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCountSettings, err := repo.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(Equal(clinics.DefaultPatientCountSettings()))
		})

		It("returns patient count settings when set", func() {
			expectedPatientCountSettings := &clinics.PatientCountSettings{
				HardLimit: &clinics.PatientCountLimit{
					PatientCount: 10,
				},
				SoftLimit: &clinics.PatientCountLimit{
					PatientCount: 5,
				},
			}

			clinic := clinicsTest.RandomClinic()
			clinic.PatientCountSettings = expectedPatientCountSettings
			clinic, err := repo.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCountSettings, err := repo.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(Equal(expectedPatientCountSettings))
		})
	})

	Describe("UpdatePatientCountSettings", func() {
		It("returns an error if the clinic is not found", func() {
			expectedPatientCountSettings := &clinics.PatientCountSettings{
				HardLimit: &clinics.PatientCountLimit{
					PatientCount: 10,
				},
				SoftLimit: &clinics.PatientCountLimit{
					PatientCount: 5,
				},
			}

			err := repo.UpdatePatientCountSettings(context.Background(), primitive.NewObjectID().Hex(), expectedPatientCountSettings)
			Expect(err).To(Equal(clinics.ErrNotFound))
		})

		It("updates the patient count settings", func() {
			clinic := clinicsTest.RandomClinic()
			clinic, err := repo.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			expectedPatientCountSettings := &clinics.PatientCountSettings{
				HardLimit: &clinics.PatientCountLimit{
					PatientCount: 10,
				},
				SoftLimit: &clinics.PatientCountLimit{
					PatientCount: 5,
				},
			}

			err = repo.UpdatePatientCountSettings(context.Background(), clinic.Id.Hex(), expectedPatientCountSettings)
			Expect(err).ToNot(HaveOccurred())

			patientCountSettings, err := repo.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(Equal(expectedPatientCountSettings))
		})
	})

	Describe("GetPatientCount", func() {
		It("returns patient count by default", func() {
			clinic := clinicsTest.RandomClinic()
			clinic, err := repo.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCount, err := repo.GetPatientCount(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCount).To(Equal(clinics.NewPatientCount()))
		})

		It("returns patient count when set", func() {
			expectedPatientCount := &clinics.PatientCount{
				PatientCount: 10,
			}

			clinic := clinicsTest.RandomClinic()
			clinic.PatientCount = expectedPatientCount
			clinic, err := repo.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCount, err := repo.GetPatientCount(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCount).To(Equal(expectedPatientCount))
		})
	})

	Describe("UpdatePatientCount", func() {
		It("returns an error if the clinic is not found", func() {
			expectedPatientCount := &clinics.PatientCount{
				PatientCount: 10,
			}

			err := repo.UpdatePatientCount(context.Background(), primitive.NewObjectID().Hex(), expectedPatientCount)
			Expect(err).To(Equal(clinics.ErrNotFound))
		})

		It("updates the patient count", func() {
			clinic := clinicsTest.RandomClinic()
			clinic, err := repo.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			expectedPatientCount := &clinics.PatientCount{
				PatientCount: 10,
			}

			err = repo.UpdatePatientCount(context.Background(), clinic.Id.Hex(), expectedPatientCount)
			Expect(err).ToNot(HaveOccurred())

			patientCount, err := repo.GetPatientCount(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCount).To(Equal(expectedPatientCount))
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

			Expect(th.Repo.CreateSite(ctx, th.Clinic.Id.Hex(), site)).To(Succeed())
		})
	})

	Describe("DeleteSite", func() {
		It("deletes the site", func() {
			ctx, th := newRepoTestHelper(GinkgoT())

			Expect(th.Repo.DeleteSite(ctx, th.Clinic.Id.Hex(), th.Site.Id.Hex())).To(Succeed())
		})
	})

	Describe("ListSites", func() {
		It("lists the sites", func() {
			ctx, th := newRepoTestHelper(GinkgoT())

			sites, err := th.Repo.ListSites(ctx, th.Clinic.Id.Hex())
			Expect(err).To(Succeed())
			Expect(len(sites)).To(Equal(1))
			Expect(sites[0].Name).To(Equal(th.Site.Name))
		})
	})

	Describe("UpdateSite", func() {
		It("updates the site", func() {
			ctx, th := newRepoTestHelper(GinkgoT())
			updatedSite := th.Site
			updatedSite.Name = "New Name"

			Expect(th.Repo.UpdateSite(ctx, th.Clinic.Id.Hex(), th.Site.Id.Hex(), updatedSite)).To(Succeed())

			// double check
			sites, err := th.Repo.ListSites(ctx, th.Clinic.Id.Hex())
			Expect(err).To(Succeed())
			Expect(len(sites)).To(Equal(1))
			Expect(sites[0].Name).To(Equal(updatedSite.Name))
		})
	})
})

func genRandomTags(n int) []clinics.PatientTag {
	tags := make([]clinics.PatientTag, n)
	for i := 0; i < n; i++ {
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
	Repo   clinics.Service
	Site   *sites.Site

	t FullGinkgoTInterface
}

func newRepoTestHelper(t FullGinkgoTInterface) (context.Context, *repoTestHelper) {
	db := dbTest.GetTestDatabase()
	lifecycle := fxtest.NewLifecycle(t)
	repo, err := clinics.NewRepository(db, zap.NewNop().Sugar(), lifecycle)
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
	if err := repo.CreateSite(ctx, clinic.Id.Hex(), site); err != nil {
		t.Fatalf("failed to create initial site: %s", err)
	}
	th.Site = site
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
