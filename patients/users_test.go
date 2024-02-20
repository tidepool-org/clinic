package patients_test

import (
	"github.com/mohae/deepcopy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
)

var _ = Describe("Users", func() {
	Describe("PopulatePatientFromUserAndProfile", func() {
		It("Populates the details from the user and profile", func() {
			patient := patients.Patient{}
			user := patientsTest.RandomUser()
			profile := patientsTest.RandomProfile()

			patients.PopulatePatientFromUserAndProfile(&patient, user, profile)

			Expect(patient.BirthDate).To(PointTo(Equal(*profile.Patient.Birthday)))
			Expect(patient.Mrn).To(PointTo(Equal(*profile.Patient.Mrn)))
			Expect(patient.FullName).To(PointTo(Equal(*profile.Patient.FullName)))
			Expect(patient.Email).To(PointTo(Equal(user.Username)))
			Expect(patient.TargetDevices).To(PointTo(ConsistOf(*profile.Patient.TargetDevices)))
		})

		It("Doesn't overwrite birthdate, mrn and fullname from the user and profile", func() {
			original := patientsTest.RandomPatient()

			patient := deepcopy.Copy(original).(patients.Patient)
			user := patientsTest.RandomUser()
			profile := patientsTest.RandomProfile()

			patients.PopulatePatientFromUserAndProfile(&patient, user, profile)

			Expect(patient.BirthDate).To(PointTo(Equal(*original.BirthDate)))
			Expect(patient.Mrn).To(PointTo(Equal(*original.Mrn)))
			Expect(patient.FullName).To(PointTo(Equal(*original.FullName)))
		})
	})
})
