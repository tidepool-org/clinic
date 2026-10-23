package patients_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/pointer"
)

var _ = Describe("GlycemicRanges", func() {
	Describe("Validate", func() {
		custom := func(units string, veryLow *float64, low, high float64,
			veryHigh *float64) patients.GlycemicRanges {

			return patients.GlycemicRanges{
				Type: patients.GlycemicRangeTypeCustom,
				Custom: patients.GlycemicRangesCustom{
					Name:  "Mine",
					Units: units,
					Thresholds: patients.GlycemicRangeThresholds{
						VeryLow:  veryLow,
						Low:      low,
						High:     high,
						VeryHigh: veryHigh,
					},
				},
			}
		}

		It("accepts unset ranges", func() {
			Expect(patients.GlycemicRanges{}.Validate()).To(Succeed())
		})

		It("accepts preset ranges", func() {
			Expect(patientsTest.RandomGlycemicRangesPreset().Validate()).To(Succeed())
		})

		It("accepts random custom ranges", func() {
			for range 100 {
				Expect(patientsTest.RandomGlycemicRangesCustom().Validate()).To(Succeed())
			}
		})

		DescribeTable("accepts valid custom ranges",
			func(ranges patients.GlycemicRanges) {
				Expect(ranges.Validate()).To(Succeed())
			},
			Entry("in mg/dL", custom("mg/dL", pointer.FromAny(54.0), 70, 180,
				pointer.FromAny(250.0))),
			Entry("in mmol/L", custom("mmol/L", pointer.FromAny(3.0), 3.9, 10,
				pointer.FromAny(13.9))),
			Entry("without optional thresholds", custom("mg/dL", nil, 70, 180, nil)),
			Entry("with one-decimal mmol/L values prone to floating point error",
				custom("mmol/L", pointer.FromAny(0.3), 0.7, 1.1, pointer.FromAny(2.3))),
		)

		DescribeTable("rejects invalid custom ranges",
			func(ranges patients.GlycemicRanges, message string) {
				err := ranges.Validate()
				Expect(err).To(MatchError(errors.BadRequest))
				Expect(err).To(MatchError(ContainSubstring(message)))
			},
			Entry("with unknown units", custom("mg", nil, 70, 180, nil),
				`unknown glycemic range units "mg"`),
			Entry("with a fractional mg/dL value", custom("mg/dL", nil, 70.5, 180, nil),
				"low must have at most 0 decimal places in mg/dL"),
			Entry("with a two-decimal mmol/L value",
				custom("mmol/L", nil, 3.9, 10, pointer.FromAny(13.91)),
				"veryHigh must have at most 1 decimal places in mmol/L"),
			Entry("with low equal to veryLow",
				custom("mg/dL", pointer.FromAny(70.0), 70, 180, nil),
				"low must be greater than veryLow"),
			Entry("with high below low", custom("mg/dL", nil, 180, 70, nil),
				"high must be greater than low"),
			Entry("with veryHigh below high",
				custom("mmol/L", nil, 3.9, 10, pointer.FromAny(9.9)),
				"veryHigh must be greater than high"),
		)
	})
})
