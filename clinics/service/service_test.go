package service_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	clinicsRepository "github.com/tidepool-org/clinic/clinics/repository"
	clinicsService "github.com/tidepool-org/clinic/clinics/service"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

func Ptr[T any](value T) *T {
	return &value
}

var _ = Describe("Clinics", func() {
	var database *mongo.Database
	var service clinics.Service

	BeforeEach(func() {
		var err error
		database = dbTest.GetTestDatabase()
		lifecycle := fxtest.NewLifecycle(GinkgoT())
		repository, err := clinicsRepository.NewRepository(database, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(repository).ToNot(BeNil())
		service, err = clinicsService.NewService(repository)
		Expect(err).ToNot(HaveOccurred())
		Expect(service).ToNot(BeNil())
		lifecycle.RequireStart()
	})

	Describe("GetPatientCountSettings", func() {
		It("returns patient count settings by default", func() {
			clinic := clinicsTest.RandomClinic()
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCountSettings, err := service.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
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
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			patientCountSettings, err := service.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
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

			err := service.UpdatePatientCountSettings(context.Background(), primitive.NewObjectID().Hex(), expectedPatientCountSettings)
			Expect(err).To(Equal(clinics.ErrNotFound))
		})

		It("updates the patient count settings", func() {
			clinic := clinicsTest.RandomClinic()
			clinic, err := service.Create(context.Background(), clinic)
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

			err = service.UpdatePatientCountSettings(context.Background(), clinic.Id.Hex(), expectedPatientCountSettings)
			Expect(err).ToNot(HaveOccurred())

			patientCountSettings, err := service.GetPatientCountSettings(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(patientCountSettings).To(Equal(expectedPatientCountSettings))
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
				PatientCount: 10,
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

	Describe("UpdatePatientCount", func() {
		It("returns an error if the clinic is not found", func() {
			expectedPatientCount := &clinics.PatientCount{
				PatientCount: 10,
			}

			err := service.UpdatePatientCount(context.Background(), primitive.NewObjectID().Hex(), expectedPatientCount)
			Expect(err).To(Equal(clinics.ErrNotFound))
		})

		It("updates the patient count", func() {
			clinic := clinicsTest.RandomClinic()
			clinic, err := service.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			expectedPatientCount := &clinics.PatientCount{
				PatientCount: 10,
			}

			err = service.UpdatePatientCount(context.Background(), clinic.Id.Hex(), expectedPatientCount)
			Expect(err).ToNot(HaveOccurred())

			patientCount, err := service.GetPatientCount(context.Background(), clinic.Id.Hex())
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
