package clinics_test

import (
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinics"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var _ = Describe("canAddPatientTag", func() {
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
