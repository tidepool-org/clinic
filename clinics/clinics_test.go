package clinics_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/test"
)

var _ = Describe("Clinics", func() {
	Describe("PatientCount", func() {
		Describe("IsValid", func() {
			var patientCount *clinics.PatientCount

			BeforeEach(func() {
				patientCount = &clinics.PatientCount{
					PatientCount: 0,
				}
			})

			It("returns true when valid", func() {
				isValid := patientCount.IsValid()
				Expect(isValid).To(BeTrue())
			})

			It("returns false when patient count is invalid", func() {
				patientCount.PatientCount = -1

				isValid := patientCount.IsValid()
				Expect(isValid).To(BeFalse())
			})
		})
	})

	Describe("PatientCountSettings", func() {
		Describe("IsValid", func() {
			var now time.Time
			var patientCountSettings *clinics.PatientCountSettings

			BeforeEach(func() {
				now = time.Now()
				patientCountSettings = &clinics.PatientCountSettings{
					HardLimit: &clinics.PatientCountLimit{
						PatientCount: 0,
						StartDate:    Ptr(now.Add(-time.Minute)),
						EndDate:      Ptr(now.Add(time.Minute)),
					},
					SoftLimit: &clinics.PatientCountLimit{
						PatientCount: 0,
						StartDate:    Ptr(now.Add(-time.Minute)),
						EndDate:      Ptr(now.Add(time.Minute)),
					},
				}
			})

			It("returns true when valid", func() {
				isValid := patientCountSettings.IsValid()
				Expect(isValid).To(BeTrue())
			})

			It("returns false when hard limit patient count is invalid", func() {
				patientCountSettings.HardLimit.PatientCount = -1

				isValid := patientCountSettings.IsValid()
				Expect(isValid).To(BeFalse())
			})

			It("returns false when soft limit patient count is invalid", func() {
				patientCountSettings.SoftLimit.PatientCount = -1

				isValid := patientCountSettings.IsValid()
				Expect(isValid).To(BeFalse())
			})
		})
	})
	Describe("PatientCountLimit", func() {
		Describe("IsValid", func() {
			var now time.Time
			var patientCountLimit *clinics.PatientCountLimit

			BeforeEach(func() {
				now = time.Now()
				patientCountLimit = &clinics.PatientCountLimit{
					PatientCount: 0,
					StartDate:    Ptr(now.Add(-time.Minute)),
					EndDate:      Ptr(now.Add(time.Minute)),
				}
			})

			It("returns true when valid", func() {
				isValid := patientCountLimit.IsValid()
				Expect(isValid).To(BeTrue())
			})

			It("returns false when patient count is invalid", func() {
				patientCountLimit.PatientCount = -1

				isValid := patientCountLimit.IsValid()
				Expect(isValid).To(BeFalse())
			})

			It("returns false when start date is after end date", func() {
				patientCountLimit.StartDate = Ptr(now.Add(2 * time.Minute))

				isValid := patientCountLimit.IsValid()
				Expect(isValid).To(BeFalse())
			})
		})
	})

	Describe("Filter By Workspace Id", func() {
		var list []*clinics.Clinic
		var index int
		var random *clinics.Clinic

		BeforeEach(func() {
			list = test.RandomClinics(10)
			index = test.Faker.Generator.Intn(10)
			random = list[index]
		})

		It("Returns the correct clinic when filtering by clinic id", func() {
			result, err := clinics.FilterByWorkspaceId(list, random.Id.Hex(), "clinicId")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Id.Hex()).To(Equal(random.Id.Hex()))
		})

		It("Returns the correct clinic when filtering by ehr source id", func() {
			result, err := clinics.FilterByWorkspaceId(list, random.EHRSettings.SourceId, "ehrSourceId")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Id.Hex()).To(Equal(random.Id.Hex()))
		})

		It("Returns error when workspace id type is empty", func() {
			_, err := clinics.FilterByWorkspaceId(list, "test", "")
			Expect(err).To(HaveOccurred())
		})

		It("Returns error when workspace id type is not support", func() {
			_, err := clinics.FilterByWorkspaceId(list, "test", "test")
			Expect(err).To(HaveOccurred())
		})
	})
})
