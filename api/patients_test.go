package api_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic/api"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/pointer"
)

var _ = DescribeTable("ParseCGMSummaryFilters",
	func(input api.ListPatientsParams, expected patients.SummaryFilters, expectedErr error) {
		got, err := api.ParseCGMSummaryFilters(input)
		if expectedErr != nil {
			Expect(err).To(MatchError(expectedErr))
		} else {
			Expect(err).ToNot(HaveOccurred())
		}
		Expect(got).To(Equal(expected))
	},
	Entry("negative value for time in range percent delta",
		api.ListPatientsParams{
			CgmTimeInTargetPercentDelta: pointer.FromAny(api.FloatFilter("<=-0.05")),
		},
		patients.SummaryFilters{
			"timeInTargetPercentDelta": {
				Cmp:   "<=",
				Value: -0.05,
			},
		},
		nil,
	),
	Entry("explicitly positive value for time in range percent delta",
		api.ListPatientsParams{
			CgmTimeInTargetPercentDelta: pointer.FromAny(api.FloatFilter("<=+0.05")),
		},
		patients.SummaryFilters{
			"timeInTargetPercentDelta": {
				Cmp:   "<=",
				Value: 0.05,
			},
		},
		nil,
	),
	Entry("implicitly positive value for time in range percent delta",
		api.ListPatientsParams{
			CgmTimeInTargetPercentDelta: pointer.FromAny(api.FloatFilter("<=0.05")),
		},
		patients.SummaryFilters{
			"timeInTargetPercentDelta": {
				Cmp:   "<=",
				Value: 0.05,
			},
		},
		nil,
	),
	Entry("four decimal places",
		api.ListPatientsParams{
			CgmTimeCGMUsePercent: pointer.FromAny(api.FloatFilter(">=0.6955")),
		},
		patients.SummaryFilters{
			"timeCGMUsePercent": {
				Cmp:   ">=",
				Value: 0.6955,
			},
		},
		nil,
	),
	Entry("ten decimal places",
		api.ListPatientsParams{
			CgmTimeCGMUsePercent: pointer.FromAny(api.FloatFilter("<0.1234567891")),
		},
		patients.SummaryFilters{
			"timeCGMUsePercent": {
				Cmp:   "<",
				Value: 0.1234567891,
			},
		},
		nil,
	),
	Entry("negative value with four decimal places",
		api.ListPatientsParams{
			CgmTimeInTargetPercentDelta: pointer.FromAny(api.FloatFilter("<=-0.1234")),
		},
		patients.SummaryFilters{
			"timeInTargetPercentDelta": {
				Cmp:   "<=",
				Value: -0.1234,
			},
		},
		nil,
	),
	Entry("value without a decimal point",
		api.ListPatientsParams{
			CgmTimeCGMUseRecords: pointer.FromAny(api.IntFilter(">=7")),
		},
		patients.SummaryFilters{
			"timeCGMUseRecords": {
				Cmp:   ">=",
				Value: 7,
			},
		},
		nil,
	),
	Entry("value with multiple integer digits",
		api.ListPatientsParams{
			CgmAverageGlucoseMmol: pointer.FromAny(api.FloatFilter("<10.5")),
		},
		patients.SummaryFilters{
			"averageGlucoseMmol": {
				Cmp:   "<",
				Value: 10.5,
			},
		},
		nil,
	),
	Entry("value without an integer digit",
		api.ListPatientsParams{
			CgmTimeCGMUsePercent: pointer.FromAny(api.FloatFilter(">.6955")),
		},
		patients.SummaryFilters{
			"timeCGMUsePercent": {
				Cmp:   ">",
				Value: 0.6955,
			},
		},
		nil,
	),
	Entry("ten integer digits",
		api.ListPatientsParams{
			CgmTimeCGMUseRecords: pointer.FromAny(api.IntFilter(">=1234567890")),
		},
		patients.SummaryFilters{
			"timeCGMUseRecords": {
				Cmp:   ">=",
				Value: 1234567890,
			},
		},
		nil,
	),
	Entry("eleven integer digits is rejected",
		api.ListPatientsParams{
			CgmTimeCGMUseRecords: pointer.FromAny(api.IntFilter(">=12345678901")),
		},
		patients.SummaryFilters{},
		errors.BadRequest,
	),
	Entry("eleven decimal places is rejected",
		api.ListPatientsParams{
			CgmTimeCGMUsePercent: pointer.FromAny(api.FloatFilter(">=0.12345678901")),
		},
		patients.SummaryFilters{},
		errors.BadRequest,
	),
	Entry("missing comparator is rejected",
		api.ListPatientsParams{
			CgmTimeCGMUsePercent: pointer.FromAny(api.FloatFilter("0.5")),
		},
		patients.SummaryFilters{},
		errors.BadRequest,
	),
	Entry("non-numeric value is rejected",
		api.ListPatientsParams{
			CgmTimeCGMUsePercent: pointer.FromAny(api.FloatFilter(">=abc")),
		},
		patients.SummaryFilters{},
		errors.BadRequest,
	),
)
