package clinics_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinics"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
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
		repo, err = clinics.NewRepository(database, lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(repo).ToNot(BeNil())
		lifecycle.RequireStart()
	})

	Describe("GetPatientCountSettings", func() {
		It("returns no patient count settings by default", func() {
			clinic := clinicsTest.RandomClinic()
			clinic, err := repo.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCountSettings, err := repo.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(BeNil())
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
		It("returns no patient count by default", func() {
			clinic := clinicsTest.RandomClinic()
			clinic, err := repo.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCount, err := repo.GetPatientCount(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCount).To(BeNil())
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
