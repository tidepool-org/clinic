package ehr_test

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"

	"github.com/tidepool-org/clinic/ehr"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/test"
)

var _ = Describe("Compute", func() {
	var summary *patients.Summary

	BeforeEach(func() {
		summary = fixtureSummary()
	})

	Context("with mmol/L preferred units", func() {
		When("icode is unset", func() {
			It("returns the formatted cgm and bgm statistics in order", func() {
				stats := ehr.Compute(summary, ehr.MmolL, false)
				Expect(stats).To(HaveExactElements(
					matchStatistic("REPORTING_PERIOD_START_CGM", "2023-04-09T17:44:09Z", "DateTime", ""),
					matchStatistic("REPORTING_PERIOD_END_CGM", "2023-04-23T17:44:09Z", "DateTime", ""),
					matchStatistic("REPORTING_PERIOD_START_CGM_DATA", "2023-04-14T00:00:00Z", "DateTime", ""),
					matchStatistic("TIME_ABOVE_RANGE_VERY_HIGH_CGM", "4.4059", "Numeric", "%"),
					matchStatistic("TIME_ABOVE_RANGE_HIGH_CGM", "25.6436", "Numeric", "%"),
					matchStatistic("TIME_IN_RANGE_CGM", "56.2871", "Numeric", "%"),
					matchStatistic("TIME_BELOW_RANGE_LOW_CGM", "8.6139", "Numeric", "%"),
					matchStatistic("TIME_BELOW_RANGE_VERY_LOW_CGM", "5.0495", "Numeric", "%"),
					matchStatistic("GLUCOSE_MANAGEMENT_INDICATOR", "6.7206", "Numeric", ""),
					matchStatistic("AVERAGE_CGM", "7.9212", "Numeric", "mmol/L"),
					matchStatistic("STANDARD_DEVIATION_CGM", "1.4697", "Numeric", "mmol/L"),
					matchStatistic("COEFFICIENT_OF_VARIATION_CGM", "0.2004", "Numeric", ""),
					matchStatistic("ACTIVE_WEAR_TIME_CGM", "50.1262", "Numeric", "%"),
					matchStatistic("DAYS_WITH_DATA_CGM", "2", "Numeric", "day"),
					matchStatistic("HOURS_WITH_DATA_CGM", "28", "Numeric", "hour"),
					matchStatistic("REPORTING_PERIOD_START_SMBG", "2023-04-11T00:57:11Z", "DateTime", ""),
					matchStatistic("REPORTING_PERIOD_END_SMBG", "2023-04-25T00:57:11Z", "DateTime", ""),
					matchStatistic("REPORTING_PERIOD_START_SMBG_DATA", "2023-04-11T00:57:11Z", "DateTime", ""),
					matchStatistic("TIME_ABOVE_RANGE_VERY_HIGH_SMBG", "18.8406", "Numeric", "%"),
					matchStatistic("TIME_ABOVE_RANGE_HIGH_SMBG", "23.1884", "Numeric", "%"),
					matchStatistic("TIME_IN_RANGE_SMBG", "44.9275", "Numeric", "%"),
					matchStatistic("TIME_BELOW_RANGE_LOW_SMBG", "7.2464", "Numeric", "%"),
					matchStatistic("TIME_BELOW_RANGE_VERY_LOW_SMBG", "5.7971", "Numeric", "%"),
					matchStatistic("READINGS_ABOVE_RANGE_VERY_HIGH_SMBG", "13", "Numeric", ""),
					matchStatistic("READINGS_BELOW_RANGE_VERY_LOW_SMBG", "4", "Numeric", ""),
					matchStatistic("MAX_SMBG", "15.5556", "Numeric", "mmol/L"),
					matchStatistic("MIN_SMBG", "2.9889", "Numeric", "mmol/L"),
					matchStatistic("AVERAGE_SMBG", "9.5634", "Numeric", "mmol/L"),
					matchStatistic("STANDARD_DEVIATION_SMBG", "1.4698", "Numeric", "mmol/L"),
					matchStatistic("COEFFICIENT_OF_VARIATION_SMBG", "0.2005", "Numeric", ""),
					matchStatistic("TOTAL_READING_COUNT_SMBG", "69", "Numeric", ""),
					matchStatistic("CHECK_RATE_READINGS_DAY_SMBG", "4.9286", "Numeric", ""),
					matchStatistic("DAYS_WITH_DATA_SMBG", "3", "Numeric", "day"),
				))
			})
		})

		When("icode is set", func() {
			It("returns the statistics with icode formatting in order", func() {
				stats := ehr.Compute(summary, ehr.MmolL, true)
				Expect(stats).To(HaveExactElements(
					matchStatistic("REPORTING_PERIOD_START_CGM", "2023-04-09T17:44:09Z", "DateTime", ""),
					matchStatistic("REPORTING_PERIOD_END_CGM", "2023-04-23T17:44:09Z", "DateTime", ""),
					matchStatistic("REPORTING_PERIOD_START_CGM_DATA", "2023-04-14T00:00:00Z", "DateTime", ""),
					matchStatistic("TIME_ABOVE_RANGE_VERY_HIGH_CGM", "4", "Numeric", "%"),
					matchStatistic("TIME_ABOVE_RANGE_HIGH_CGM", "26", "Numeric", "%"),
					matchStatistic("TIME_IN_RANGE_CGM", "56", "Numeric", "%"),
					matchStatistic("TIME_BELOW_RANGE_LOW_CGM", "9", "Numeric", "%"),
					matchStatistic("TIME_BELOW_RANGE_VERY_LOW_CGM", "5", "Numeric", "%"),
					matchStatistic("GLUCOSE_MANAGEMENT_INDICATOR", "6.7", "Numeric", ""),
					matchStatistic("AVERAGE_CGM", "7.9", "Numeric", "mmol/L"),
					matchStatistic("STANDARD_DEVIATION_CGM", "1.5", "Numeric", "mmol/L"),
					matchStatistic("COEFFICIENT_OF_VARIATION_CGM", "20.0", "Numeric", "%"),
					matchStatistic("ACTIVE_WEAR_TIME_CGM", "50.13", "Numeric", "%"),
					matchStatistic("DAYS_WITH_DATA_CGM", "2", "Numeric", "day"),
					matchStatistic("HOURS_WITH_DATA_CGM", "28", "Numeric", "hour"),
					matchStatistic("REPORTING_PERIOD_START_SMBG", "2023-04-11T00:57:11Z", "DateTime", ""),
					matchStatistic("REPORTING_PERIOD_END_SMBG", "2023-04-25T00:57:11Z", "DateTime", ""),
					matchStatistic("REPORTING_PERIOD_START_SMBG_DATA", "2023-04-11T00:57:11Z", "DateTime", ""),
					matchStatistic("TIME_ABOVE_RANGE_VERY_HIGH_SMBG", "19", "Numeric", "%"),
					matchStatistic("TIME_ABOVE_RANGE_HIGH_SMBG", "23", "Numeric", "%"),
					matchStatistic("TIME_IN_RANGE_SMBG", "45", "Numeric", "%"),
					matchStatistic("TIME_BELOW_RANGE_LOW_SMBG", "7", "Numeric", "%"),
					matchStatistic("TIME_BELOW_RANGE_VERY_LOW_SMBG", "6", "Numeric", "%"),
					matchStatistic("READINGS_ABOVE_RANGE_VERY_HIGH_SMBG", "13", "Numeric", ""),
					matchStatistic("READINGS_BELOW_RANGE_VERY_LOW_SMBG", "4", "Numeric", ""),
					matchStatistic("MAX_SMBG", "15.6", "Numeric", "mmol/L"),
					matchStatistic("MIN_SMBG", "3.0", "Numeric", "mmol/L"),
					matchStatistic("AVERAGE_SMBG", "9.6", "Numeric", "mmol/L"),
					matchStatistic("STANDARD_DEVIATION_SMBG", "1.5", "Numeric", "mmol/L"),
					matchStatistic("COEFFICIENT_OF_VARIATION_SMBG", "20.0", "Numeric", "%"),
					matchStatistic("TOTAL_READING_COUNT_SMBG", "69", "Numeric", ""),
					matchStatistic("CHECK_RATE_READINGS_DAY_SMBG", "4.9286", "Numeric", ""),
					matchStatistic("DAYS_WITH_DATA_SMBG", "3", "Numeric", "day"),
				))
			})
		})
	})

	Context("with mg/dL preferred units", func() {
		It("converts blood glucose values with icode set", func() {
			stats := ehr.Compute(summary, ehr.MgdL, true)
			Expect(stats).To(ContainElement(matchStatistic("AVERAGE_CGM", "143", "Numeric", "mg/dL")))
			Expect(stats).To(ContainElement(matchStatistic("AVERAGE_SMBG", "172", "Numeric", "mg/dL")))
			Expect(stats).To(ContainElement(matchStatistic("STANDARD_DEVIATION_CGM", "26.5", "Numeric", "mg/dL")))
			Expect(stats).To(ContainElement(matchStatistic("STANDARD_DEVIATION_SMBG", "26.5", "Numeric", "mg/dL")))
			Expect(stats).To(ContainElement(matchStatistic("MIN_SMBG", "54", "Numeric", "mg/dL")))
			Expect(stats).To(ContainElement(matchStatistic("MAX_SMBG", "280", "Numeric", "mg/dL")))
		})

		It("converts blood glucose values with icode unset", func() {
			stats := ehr.Compute(summary, ehr.MgdL, false)
			Expect(stats).To(ContainElement(matchStatistic("AVERAGE_CGM", "142.7052", "Numeric", "mg/dL")))
			Expect(stats).To(ContainElement(matchStatistic("AVERAGE_SMBG", "172.2908", "Numeric", "mg/dL")))
			Expect(stats).To(ContainElement(matchStatistic("STANDARD_DEVIATION_CGM", "26.4774", "Numeric", "mg/dL")))
			Expect(stats).To(ContainElement(matchStatistic("STANDARD_DEVIATION_SMBG", "26.4792", "Numeric", "mg/dL")))
			Expect(stats).To(ContainElement(matchStatistic("MIN_SMBG", "53.8464", "Numeric", "mg/dL")))
			Expect(stats).To(ContainElement(matchStatistic("MAX_SMBG", "280.2426", "Numeric", "mg/dL")))
		})
	})

	It("returns no statistics for a nil summary", func() {
		Expect(ehr.Compute(nil, ehr.MmolL, false)).To(BeEmpty())
	})

	It("returns no statistics for an empty summary", func() {
		Expect(ehr.Compute(&patients.Summary{}, ehr.MmolL, false)).To(BeEmpty())
	})

	It("omits the bgm block when only cgm data is present", func() {
		stats := ehr.Compute(&patients.Summary{CGM: summary.CGM}, ehr.MmolL, false)
		Expect(stats).To(HaveLen(15))
		Expect(stats[0].Code).To(Equal("REPORTING_PERIOD_START_CGM"))
		for _, s := range stats {
			Expect(s.Code).ToNot(ContainSubstring("SMBG"))
		}
	})

	DescribeTable("gates out a block when its dates are unavailable",
		func(dates patients.PatientSummaryDates) {
			gated := &patients.Summary{
				CGM: &patients.PatientCGMStats{
					Dates:   dates,
					Periods: patients.PatientCGMPeriods{"14d": {AverageGlucoseMmol: f(7)}},
				},
			}
			Expect(ehr.Compute(gated, ehr.MmolL, false)).To(BeEmpty())
		},
		Entry("missing last updated date", patients.PatientSummaryDates{LastData: tp("2023-04-23T17:44:09Z")}),
		Entry("missing last data", patients.PatientSummaryDates{LastUpdatedDate: tp("2023-06-22T23:44:16Z")}),
		Entry("zero last updated date", patients.PatientSummaryDates{LastUpdatedDate: &time.Time{}, LastData: tp("2023-04-23T17:44:09Z")}),
		Entry("zero last data", patients.PatientSummaryDates{LastUpdatedDate: tp("2023-06-22T23:44:16Z"), LastData: &time.Time{}}),
	)

	It("drops unavailable metrics when the 14d period is missing", func() {
		noPeriod := &patients.Summary{
			CGM: &patients.PatientCGMStats{
				Dates:   summary.CGM.Dates,
				Periods: patients.PatientCGMPeriods{"7d": {AverageGlucoseMmol: f(7)}},
			},
		}

		stats := ehr.Compute(noPeriod, ehr.MmolL, false)
		// Only the three reporting-period dates survive; numeric metrics are dropped.
		Expect(stats).To(HaveLen(3))
		for _, s := range stats {
			Expect(s.Kind).To(Equal(ehr.KindDate))
		}
	})

	It("sets the kind and display of each statistic", func() {
		stats := ehr.Compute(summary, ehr.MmolL, false)
		Expect(stats).To(ContainElement(MatchFields(IgnoreExtras, Fields{
			"Code": Equal("REPORTING_PERIOD_START_CGM"),
			"Kind": Equal(ehr.KindDate),
		})))
		Expect(stats).To(ContainElement(MatchFields(IgnoreExtras, Fields{
			"Code": Equal("DAYS_WITH_DATA_CGM"),
			"Kind": Equal(ehr.KindInteger),
		})))
		Expect(stats).To(ContainElement(MatchFields(IgnoreExtras, Fields{
			"Code":    Equal("TIME_IN_RANGE_CGM"),
			"Kind":    Equal(ehr.KindDecimal),
			"Display": Equal("Time In Range CGM"),
		})))
		Expect(stats).To(ContainElement(MatchFields(IgnoreExtras, Fields{
			"Code":    Equal("GLUCOSE_MANAGEMENT_INDICATOR"),
			"Display": Equal("Glucose Management Indicator"),
		})))
	})
})

// matchStatistic matches a statistic by code, formatted value, coarse value
// type ("Numeric"/"DateTime") and unit, and requires a non-empty reporting
// time.
func matchStatistic(code, value, valueType, unit string) types.GomegaMatcher {
	return SatisfyAll(
		MatchFields(IgnoreExtras, Fields{
			"Code":     Equal(code),
			"Value":    Equal(value),
			"Unit":     Equal(unit),
			"DateTime": Not(BeEmpty()),
		}),
		WithTransform(ehr.Statistic.ValueType, Equal(valueType)),
	)
}

func f(v float64) *float64 { return &v }
func tp(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

// fixtureSummary loads a patient summary reproducing clinic-worker's
// subscriptionmatchresponse.json 14d periods, so this package is the
// regression gate for the (unchanged) Redox flowsheet wire output.
func fixtureSummary() *patients.Summary {
	data, err := test.LoadFixture("test/fixtures/summary.json")
	Expect(err).ToNot(HaveOccurred())

	summary := &patients.Summary{}
	Expect(json.Unmarshal(data, summary)).To(Succeed())
	return summary
}
