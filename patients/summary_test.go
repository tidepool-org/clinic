package patients_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/patients"
)

func ptr[T any](v T) *T {
	return &v
}

func generateDefaultRiskySummary() (userSummary patients.Summary) {
	userSummary = patients.Summary{
		CGM: &patients.PatientCGMStats{
			Periods: &patients.PatientCGMPeriods{
				"1d": patients.PatientCGMPeriod{
					TimeInVeryLowPercent: ptr(0.02),
				},
				"7d": patients.PatientCGMPeriod{
					TimeInLowPercent: ptr(0.05),
				},
				"14d": patients.PatientCGMPeriod{
					TimeInTargetPercentDelta: ptr(-0.2),
				},
				"30d": patients.PatientCGMPeriod{
					TimeInTargetPercent: ptr(0.6),
				},
				"60d": patients.PatientCGMPeriod{
					TimeCGMUsePercent: ptr(0.6),
				},
			},
		},
	}

	return
}

func generateDefaultDoubleRiskySummary() (userSummary patients.Summary) {
	userSummary = patients.Summary{
		CGM: &patients.PatientCGMStats{
			Periods: &patients.PatientCGMPeriods{
				"1d": patients.PatientCGMPeriod{
					TimeInVeryLowPercent: ptr(0.02),
					TimeInLowPercent:     ptr(0.05),
				},
				"7d": patients.PatientCGMPeriod{
					TimeInLowPercent:         ptr(0.05),
					TimeInTargetPercentDelta: ptr(-0.2),
				},
				"14d": patients.PatientCGMPeriod{
					TimeInTargetPercentDelta: ptr(-0.2),
					TimeInTargetPercent:      ptr(0.6),
				},
				"30d": patients.PatientCGMPeriod{
					TimeInTargetPercent: ptr(0.6),
					TimeCGMUsePercent:   ptr(0.6),
				},
				"60d": patients.PatientCGMPeriod{
					TimeCGMUsePercent:    ptr(0.6),
					TimeInVeryLowPercent: ptr(0.02),
				},
			},
		},
	}

	return
}

var _ = Describe("Patients Service", func() {
	Describe("TIDE Report", func() {
		Context("Categorization", func() {
			It("A summary with the default report and no competing categories", func() {
				config := patients.DefaultTideReport()
				userSummary := generateDefaultRiskySummary()
				userSummary.TideCategorize([]*patients.TideFilters{&config})

				Expect(userSummary.Risk).ToNot(BeNil())

				Expect(userSummary.Risk).To(HaveKey("1d"))
				Expect(userSummary.Risk).To(HaveKey("7d"))
				Expect(userSummary.Risk).To(HaveKey("14d"))
				Expect(userSummary.Risk).To(HaveKey("30d"))
				Expect(userSummary.Risk).To(HaveKey("60d"))

				Expect((*userSummary.Risk["1d"])[0]).To(Equal(*config[0].Id))
				Expect((*userSummary.Risk["7d"])[0]).To(Equal(*config[1].Id))
				Expect((*userSummary.Risk["14d"])[0]).To(Equal(*config[2].Id))
				Expect((*userSummary.Risk["30d"])[0]).To(Equal(*config[3].Id))
				Expect((*userSummary.Risk["60d"])[0]).To(Equal(*config[4].Id))
			})

			It("A summary with the default report and competing categories", func() {
				config := patients.DefaultTideReport()
				userSummary := generateDefaultDoubleRiskySummary()
				userSummary.TideCategorize([]*patients.TideFilters{&config})

				Expect(userSummary.Risk).ToNot(BeNil())

				Expect(userSummary.Risk).To(HaveKey("1d"))
				Expect(*userSummary.Risk["1d"]).To(HaveLen(1))

				Expect(userSummary.Risk).To(HaveKey("7d"))
				Expect(*userSummary.Risk["7d"]).To(HaveLen(1))

				Expect(userSummary.Risk).To(HaveKey("14d"))
				Expect(*userSummary.Risk["14d"]).To(HaveLen(1))

				Expect(userSummary.Risk).To(HaveKey("30d"))
				Expect(*userSummary.Risk["30d"]).To(HaveLen(1))

				Expect(userSummary.Risk).To(HaveKey("60d"))
				Expect(*userSummary.Risk["60d"]).To(HaveLen(1))

				Expect((*userSummary.Risk["1d"])[0]).To(Equal(*config[0].Id))
				Expect((*userSummary.Risk["7d"])[0]).To(Equal(*config[1].Id))
				Expect((*userSummary.Risk["14d"])[0]).To(Equal(*config[2].Id))
				Expect((*userSummary.Risk["30d"])[0]).To(Equal(*config[3].Id))
				Expect((*userSummary.Risk["60d"])[0]).To(Equal(*config[0].Id))
			})
		})
	})
})
