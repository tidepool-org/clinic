package clinics_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/test"
)

var _ = Describe("Clinics", func() {
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
