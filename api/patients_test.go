package api_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic/api"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/pointer"
	"github.com/tidepool-org/clinic/test"
)

func TestSuite(t *testing.T) {
	test.Test(t)
}

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
)
