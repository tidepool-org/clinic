package xealth_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/tidepool-org/clinic/ehr"
	"github.com/tidepool-org/clinic/xealth"
	"github.com/tidepool-org/clinic/xealth_client"
)

var _ = Describe("NewSummaryStatsObservation", func() {
	reportingTime, _ := time.Parse(time.RFC3339, "2024-01-18T09:44:11Z")
	orderId := "7e316617-ef33-4859-b0c9-36bddbfe9229"

	stats := []ehr.Statistic{
		{Code: "REPORTING_PERIOD_START_CGM", Display: "Reporting Period Start CGM", Value: "2024-01-18T09:44:11Z", Kind: ehr.KindDate},
		{Code: "DAYS_WITH_DATA_CGM", Display: "Days With Data CGM", Value: "14", Unit: "day", Kind: ehr.KindInteger},
		{Code: "TOTAL_READING_COUNT_SMBG", Display: "Total Reading Count SMBG", Value: "69", Unit: "", Kind: ehr.KindInteger},
		{Code: "TIME_IN_RANGE_CGM", Display: "Time In Range CGM", Value: "74.9167", Unit: "%", Kind: ehr.KindDecimal},
		{Code: "AVERAGE_CGM", Display: "Average CGM", Value: "142.7052", Unit: "mg/dL", Kind: ehr.KindDecimal},
		{Code: "GLUCOSE_MANAGEMENT_INDICATOR", Display: "Glucose Management Indicator", Value: "6.7206", Unit: "", Kind: ehr.KindDecimal},
	}

	var observation xealth_client.GeneralObservation

	BeforeEach(func() {
		observation = xealth.NewSummaryStatsObservation(stats, orderId, reportingTime)
	})

	codingCode := func(c xealth_client.ObservationComponent) string {
		Expect(c.Code.Coding).ToNot(BeNil())
		Expect(*c.Code.Coding).ToNot(BeEmpty())
		return (*c.Code.Coding)[0].Code
	}

	It("sets the required General Observation envelope", func() {
		Expect(observation.ResourceType).To(Equal("Observation"))
		Expect(observation.Status).To(Equal("final"))
		Expect(observation.Meta.Profile).To(ConsistOf(xealth.XealthObservationGeneralProfile))
		Expect(observation.BasedOn).To(HaveLen(1))
		Expect(observation.BasedOn[0].Reference).To(Equal("ServiceRequest/" + orderId))
		Expect(observation.EffectiveDateTime).To(Equal(reportingTime))
		Expect(observation.Code.Coding).ToNot(BeNil())
		Expect(*observation.Code.Coding).To(HaveLen(1))
		Expect((*observation.Code.Coding)[0].System).To(Equal(xealth.TidepoolObservationSystem))
	})

	It("carries the ehr-order-id extension alongside basedOn", func() {
		Expect(observation.Extension).ToNot(BeNil())
		Expect(*observation.Extension).To(ConsistOf(xealth_client.ObservationExtension{
			Url:         xealth.XealthEHROrderIdExtension,
			ValueString: orderId,
		}))
	})

	It("builds one component per statistic", func() {
		Expect(observation.Component).To(HaveLen(len(stats)))
	})

	componentByCode := func(code string) xealth_client.ObservationComponent {
		for _, c := range observation.Component {
			if codingCode(c) == code {
				return c
			}
		}
		Fail("component not found: " + code)
		return xealth_client.ObservationComponent{}
	}

	It("maps dates to valueDateTime", func() {
		c := componentByCode("REPORTING_PERIOD_START_CGM")
		Expect(c.ValueDateTime).To(PointTo(Equal("2024-01-18T09:44:11Z")))
		Expect(c.ValueInteger).To(BeNil())
		Expect(c.ValueQuantity).To(BeNil())
	})

	It("maps unitless counts to valueInteger", func() {
		c := componentByCode("TOTAL_READING_COUNT_SMBG")
		Expect(c.ValueInteger).To(PointTo(Equal(69)))
		Expect(c.ValueQuantity).To(BeNil())
	})

	It("maps counts with units to a valueQuantity carrying the unit", func() {
		c := componentByCode("DAYS_WITH_DATA_CGM")
		Expect(c.ValueInteger).To(BeNil())
		Expect(c.ValueQuantity).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Value": Equal(float64(14)),
			"Unit":  PointTo(Equal("day")),
		})))
	})

	It("maps percentages to a valueQuantity with the unit", func() {
		c := componentByCode("TIME_IN_RANGE_CGM")
		Expect(c.ValueQuantity).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Value":  Equal(74.9167),
			"Unit":   PointTo(Equal("%")),
			"System": BeNil(),
			"Code":   BeNil(),
		})))
	})

	It("maps glucose to a valueQuantity in the destination unit", func() {
		c := componentByCode("AVERAGE_CGM")
		Expect(c.ValueQuantity).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Value":  Equal(142.7052),
			"Unit":   PointTo(Equal("mg/dL")),
			"System": BeNil(),
			"Code":   BeNil(),
		})))
	})

	It("maps unitless floats to a bare valueQuantity", func() {
		c := componentByCode("GLUCOSE_MANAGEMENT_INDICATOR")
		Expect(c.ValueQuantity).ToNot(BeNil())
		Expect(c.ValueQuantity.Value).To(Equal(6.7206))
		Expect(c.ValueQuantity.Unit).To(BeNil())
		Expect(c.ValueQuantity.System).To(BeNil())
		Expect(c.ValueQuantity.Code).To(BeNil())
	})

	DescribeTable("propagates the statistic code and display to the component coding",
		func(code, display string) {
			obs := xealth.NewSummaryStatsObservation([]ehr.Statistic{
				{Code: code, Display: display, Value: "1", Kind: ehr.KindDecimal, Unit: ""},
			}, orderId, reportingTime)
			Expect(obs.Component).To(HaveLen(1))
			coding := (*obs.Component[0].Code.Coding)[0]
			Expect(coding.Code).To(Equal(code))
			Expect(coding.Display).To(PointTo(Equal(display)))
		},
		Entry("CGM metric", "TIME_IN_RANGE_CGM", "Time In Range CGM"),
		Entry("SMBG metric", "TOTAL_READING_COUNT_SMBG", "Total Reading Count SMBG"),
		Entry("GMI metric", "GLUCOSE_MANAGEMENT_INDICATOR", "Glucose Management Indicator"),
	)
})
