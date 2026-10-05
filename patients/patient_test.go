package patients_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic/patients"
)

var _ = Describe("Patient", func() {
	Describe("HasEmail", func() {
		It("is false without an email", func() {
			Expect(patients.Patient{}.HasEmail()).To(BeFalse())
		})

		It("is false for an empty email", func() {
			Expect(patients.Patient{Email: strp("")}.HasEmail()).To(BeFalse())
		})

		It("is true for a non-empty email", func() {
			Expect(patients.Patient{Email: strp("a@example.com")}.HasEmail()).To(BeTrue())
		})
	})
})
