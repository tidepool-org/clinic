package xealth_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/xealth"
)

var _ = Describe("GetProvidersFromOrder", func() {
	abbott, dexcom := patients.AbbottDataSourceProviderName,
		patients.DexcomDataSourceProviderName

	DescribeTable("guardian",
		func(guardian xealth.Guardian, expected []string) {
			data := &xealth.PreorderFormData{Guardian: &guardian}
			Expect(xealth.GetProvidersFromOrder(data)).To(Equal(expected))
		},
		Entry("with an email",
			xealth.Guardian{
				Email: "a@example.com", ConnectAbbott: true, ConnectDexcom: true,
			},
			[]string{abbott, dexcom}),
		Entry("without an email",
			xealth.Guardian{ConnectAbbott: true, ConnectDexcom: true},
			[]string{}),
		Entry("with a blank email",
			xealth.Guardian{Email: " ", ConnectAbbott: true, ConnectDexcom: true},
			[]string{}),
		Entry("with an email but no providers",
			xealth.Guardian{Email: "a@example.com"},
			[]string{}),
	)

	DescribeTable("patient",
		func(patient xealth.Patient, expected []string) {
			data := &xealth.PreorderFormData{Patient: &patient}
			Expect(xealth.GetProvidersFromOrder(data)).To(Equal(expected))
		},
		Entry("with an email",
			xealth.Patient{
				Email: "a@example.com", ConnectAbbott: true, ConnectDexcom: true,
			},
			[]string{abbott, dexcom}),
		Entry("without an email",
			xealth.Patient{ConnectAbbott: true, ConnectDexcom: true},
			[]string{}),
		Entry("with a blank email",
			xealth.Patient{Email: " ", ConnectAbbott: true, ConnectDexcom: true},
			[]string{}),
	)
})
