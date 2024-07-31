package merge_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
)

var _ = Describe("MembershipRestrictionsMergePlan", func() {
	It("can be executed when source and target restrictions are not set", func() {
		plan := merge.MembershipRestrictionsMergePlan{
			SourceValue: nil,
			TargetValue: nil,
		}
		Expect(plan.PreventsMerge()).To(BeFalse())
	})

	It("can be executed when source and target restrictions are the same", func() {
		plan := merge.MembershipRestrictionsMergePlan{
			SourceValue: []clinics.MembershipRestrictions{
				{EmailDomain: "test.com", RequiredIdp: "test_idp"},
				{EmailDomain: "example.com", RequiredIdp: "example_idp"},
			},
			TargetValue: []clinics.MembershipRestrictions{
				{EmailDomain: "test.com", RequiredIdp: "test_idp"},
				{EmailDomain: "example.com", RequiredIdp: "example_idp"},
			},
		}
		Expect(plan.PreventsMerge()).To(BeFalse())
	})

	It("can be executed when the source restrictions are not set", func() {
		plan := merge.MembershipRestrictionsMergePlan{
			SourceValue: nil,
			TargetValue: []clinics.MembershipRestrictions{
				{EmailDomain: "test.com", RequiredIdp: "test_idp"},
				{EmailDomain: "example.com", RequiredIdp: "example_idp"},
			},
		}
		Expect(plan.PreventsMerge()).To(BeFalse())
	})

	It("can be executed when the source restrictions are a subset of the target", func() {
		plan := merge.MembershipRestrictionsMergePlan{
			SourceValue: []clinics.MembershipRestrictions{
				{EmailDomain: "test.com", RequiredIdp: "test_idp"},
			},
			TargetValue: []clinics.MembershipRestrictions{
				{EmailDomain: "test.com", RequiredIdp: "test_idp"},
				{EmailDomain: "example.com", RequiredIdp: "example_idp"},
			},
		}
		Expect(plan.PreventsMerge()).To(BeFalse())
	})

	It("cannot be executed when the source restrictions are not subset of the target", func() {
		plan := merge.MembershipRestrictionsMergePlan{
			SourceValue: []clinics.MembershipRestrictions{
				{EmailDomain: "a.com", RequiredIdp: "test_idp"},
			},
			TargetValue: []clinics.MembershipRestrictions{
				{EmailDomain: "test.com", RequiredIdp: "test_idp"},
				{EmailDomain: "example.com", RequiredIdp: "example_idp"},
			},
		}
		Expect(plan.PreventsMerge()).To(BeTrue())
	})
})
