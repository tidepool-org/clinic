package xealth_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/pointer"
	"github.com/tidepool-org/clinic/xealth"
)

var _ = Describe("GenerateReportUrl", func() {
	var patient patients.Patient
	var clinic clinics.Clinic

	BeforeEach(func() {
		patient = patients.Patient{
			UserId:    pointer.FromAny("1234567890"),
			BirthDate: pointer.FromAny("2000-01-01"),
			Mrn:       pointer.FromAny("mrn"),
			FullName:  pointer.FromAny("Test Patient"),
		}
		clinicId := primitive.NewObjectID()
		clinic = clinics.Clinic{Id: &clinicId}
	})

	glycemicRangeParams := func() map[string][]string {
		u, err := xealth.GenerateReportUrl("https://example.test", "token", patient, clinic)
		Expect(err).ToNot(HaveOccurred())
		q := u.Query()
		return map[string][]string{
			"glycemicRangeType":       q["glycemicRangeType"],
			"glycemicRangePreset":     q["glycemicRangePreset"],
			"glycemicRangeThresholds": q["glycemicRangeThresholds"],
		}
	}

	It("includes preset glycemic ranges", func() {
		patient.GlycemicRanges = patients.GlycemicRanges{
			Type:   patients.GlycemicRangeTypePreset,
			Preset: "adaHighRisk",
		}
		Expect(glycemicRangeParams()).To(Equal(map[string][]string{
			"glycemicRangeType":       {"preset"},
			"glycemicRangePreset":     {"adaHighRisk"},
			"glycemicRangeThresholds": nil,
		}))
	})

	It("includes all custom glycemic range thresholds", func() {
		patient.GlycemicRanges = patients.GlycemicRanges{
			Type: patients.GlycemicRangeTypeCustom,
			Custom: patients.GlycemicRangesCustom{
				Name:  "Mine, all mine",
				Units: "mg/dL",
				Thresholds: patients.GlycemicRangeThresholds{
					VeryLow:  pointer.FromAny(float64(54)),
					Low:      70,
					High:     180,
					VeryHigh: pointer.FromAny(float64(250)),
				},
			},
		}
		Expect(glycemicRangeParams()).To(Equal(map[string][]string{
			"glycemicRangeType":   {"custom"},
			"glycemicRangePreset": nil,
			"glycemicRangeThresholds": {`name,"Mine, all mine",units,mg/dL,` +
				"veryLow,54.000000,low,70.000000,high,180.000000,veryHigh,250.000000"},
		}))
	})

	It("omits unset optional custom glycemic range thresholds", func() {
		patient.GlycemicRanges = patients.GlycemicRanges{
			Type: patients.GlycemicRangeTypeCustom,
			Custom: patients.GlycemicRangesCustom{
				Name:  "Mine",
				Units: "mmol/L",
				Thresholds: patients.GlycemicRangeThresholds{
					Low:  3.9,
					High: 10,
				},
			},
		}
		Expect(glycemicRangeParams()).To(Equal(map[string][]string{
			"glycemicRangeType":   {"custom"},
			"glycemicRangePreset": nil,
			"glycemicRangeThresholds": {
				"name,Mine,units,mmol/L,low,3.900000,high,10.000000"},
		}))
	})
})
