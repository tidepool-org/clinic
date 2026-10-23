package patients_test

import (
	"math"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/pointer"
)

var _ = Describe("GlycemicRanges", func() {
	Describe("Key", func() {
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
		preset := func(name string) patients.GlycemicRanges {
			return patients.GlycemicRanges{
				Type:   patients.GlycemicRangeTypePreset,
				Preset: patients.GlycemicRangesPreset(name),
			}
		}

		DescribeTable("derives the key from the definition",
			func(ranges patients.GlycemicRanges, key string) {
				Expect(ranges.Key()).To(Equal(key))
			},
			Entry("unset ranges", patients.GlycemicRanges{}, "preset:adaStandard"),
			Entry("adaStandard", preset("adaStandard"), "preset:adaStandard"),
			Entry("adaPregnancyType1", preset("adaPregnancyType1"),
				"preset:adaPregnancyType1"),
			Entry("adaPregnancyType2", preset("adaPregnancyType2"),
				"preset:adaPregnancyType2"),
			Entry("adaHighRisk", preset("adaHighRisk"), "preset:adaHighRisk"),
			Entry("custom mg/dL with all thresholds",
				custom("mg/dL", pointer.FromAny(54.0), 70, 180, pointer.FromAny(250.0)),
				"custom:mg/dL:54,70,180,250"),
			Entry("custom mmol/L without optional thresholds",
				custom("mmol/L", nil, 3.9, 10, nil), "custom:mmol/L:,3.9,10,"),
			Entry("custom with only veryLow",
				custom("mmol/L", pointer.FromAny(3.0), 3.9, 10, nil),
				"custom:mmol/L:3,3.9,10,"),
			Entry("custom with only veryHigh",
				custom("mg/dL", nil, 70, 180, pointer.FromAny(250.0)),
				"custom:mg/dL:,70,180,250"),
			Entry("custom with a negative zero threshold",
				custom("mg/dL", pointer.FromAny(math.Copysign(0, -1)), 70, 180, nil),
				"custom:mg/dL:0,70,180,"),
		)

		It("ignores the custom range's name", func() {
			a := custom("mg/dL", nil, 70, 180, nil)
			b := a
			b.Custom.Name = "Yours"
			Expect(a.Key()).To(Equal(b.Key()))
		})

		It("changes when a single threshold changes", func() {
			a := custom("mg/dL", nil, 70, 180, nil)
			b := a
			b.Custom.Thresholds.High = 181
			Expect(a.Key()).ToNot(Equal(b.Key()))
		})
	})

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
