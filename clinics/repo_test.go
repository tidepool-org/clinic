package clinics

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/test"
)

func TestSuite(t *testing.T) {
	test.Test(t)
}

var _ = Describe("canAddPatientTag", func() {
	It("returns an error when tags exceed the maximum value", func() {
		clinicWithMaxTags := Clinic{
			PatientTags: genRandomTags(MaximumPatientTags),
		}

		err := assertCanAddPatientTag(clinicWithMaxTags, PatientTag{})

		Expect(err).To(MatchError(ErrMaximumPatientTagsExceeded))
	})

	It("returns an error when the tag to be added is a duplicate", func() {
		tagName := "first"
		firstTag := PatientTag{Name: tagName, Id: ptr(primitive.NewObjectID())}
		clinicWithDupTag := Clinic{PatientTags: []PatientTag{firstTag}}

		dupTag := PatientTag{Name: tagName, Id: ptr(primitive.NewObjectID())}
		err := assertCanAddPatientTag(clinicWithDupTag, dupTag)

		Expect(err).To(MatchError(ErrDuplicatePatientTagName))
	})
})

func genRandomTags(n int) []PatientTag {
	tags := make([]PatientTag, n)
	for i := 0; i < n; i++ {
		tags[i] = PatientTag{
			Name: fmt.Sprintf("tag%d", i),
		}
	}
	return tags
}

func ptr[A any](a A) *A {
	return &a
}
