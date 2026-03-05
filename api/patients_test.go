package api

import (
	"reflect"
	"testing"

	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/pointer"
)

func TestParseCGMSummaryFilters(t *testing.T) {
	type test struct {
		Name  string
		Input ListPatientsParams
		Exp   patients.SummaryFilters
		Err   error
	}

	tests := []test{
		{
			Name: "negative value for time in range percent delta",
			Input: ListPatientsParams{
				CgmTimeInTargetPercentDelta: pointer.FromAny(FloatFilter("<=-0.05")),
			},
			Exp: patients.SummaryFilters{
				"timeInTargetPercentDelta": {
					Cmp:   "<=",
					Value: -0.05,
				},
			},
			Err: nil,
		},
		{
			Name: "explicitly positive value for time in range percent delta",
			Input: ListPatientsParams{
				CgmTimeInTargetPercentDelta: pointer.FromAny(FloatFilter("<=+0.05")),
			},
			Exp: patients.SummaryFilters{
				"timeInTargetPercentDelta": {
					Cmp:   "<=",
					Value: 0.05,
				},
			},
			Err: nil,
		},
		{
			Name: "implicitly positive value for time in range percent delta",
			Input: ListPatientsParams{
				CgmTimeInTargetPercentDelta: pointer.FromAny(FloatFilter("<=0.05")),
			},
			Exp: patients.SummaryFilters{
				"timeInTargetPercentDelta": {
					Cmp:   "<=",
					Value: 0.05,
				},
			},
			Err: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(tt *testing.T) {
			got, err := ParseCGMSummaryFilters(test.Input)
			if err != nil && test.Err == nil {
				tt.Fatalf("expected nil error, got %s", err)
			}
			if test.Err != nil && err == nil {
				tt.Fatalf("expected error %s, got nil", test.Err)
			}
			if !reflect.DeepEqual(got, test.Exp) {
				tt.Errorf("expected %+v, got %+v", test.Exp, got)
			}
		})
	}
}
