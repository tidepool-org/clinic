package xealth

import (
	"strconv"
	"time"

	"github.com/tidepool-org/clinic/ehr"
	"github.com/tidepool-org/clinic/xealth_client"
)

const (
	// XealthObservationGeneralProfile is the FHIR profile for a General
	// Observation in the Xealth FHIR store.
	XealthObservationGeneralProfile = "https://fhir.xealth.io/StructureDefinition/xealth-observation-general"

	// TidepoolObservationSystem identifies Tidepool as the source of the
	// summary-statistic metric codes.
	TidepoolObservationSystem = "https://tidepool.org"

	// XealthEHROrderIdExtension is the Xealth extension URL that ties the
	// Observation to the originating order via the raw order id (the upstream
	// General Observation carries this alongside basedOn).
	XealthEHROrderIdExtension = "https://fhir.xealth.io/StructureDefinition/extension-ehr-order-id"

	observationStatusFinal = "final"

	// tidepoolSummaryStatsCode is the top-level Observation code identifying a
	// Tidepool summary-statistics writeback.
	tidepoolSummaryStatsCode    = "SUMMARY_STATISTICS"
	tidepoolSummaryStatsDisplay = "Tidepool Summary Statistics"
)

func NewSummaryStatsObservation(stats []ehr.Statistic, orderId string, effectiveTime time.Time) xealth_client.GeneralObservation {
	return xealth_client.GeneralObservation{
		ResourceType:      "Observation",
		Meta:              xealth_client.ObservationMeta{Profile: []string{XealthObservationGeneralProfile}},
		Status:            observationStatusFinal,
		BasedOn:           []xealth_client.ObservationReference{{Reference: "ServiceRequest/" + orderId}},
		Extension:         &[]xealth_client.ObservationExtension{{Url: XealthEHROrderIdExtension, ValueString: orderId}},
		Code:              codeableConcept(tidepoolSummaryStatsCode, tidepoolSummaryStatsDisplay),
		EffectiveDateTime: effectiveTime,
		Component:         observationComponents(stats),
	}
}

func observationComponents(stats []ehr.Statistic) []xealth_client.ObservationComponent {
	components := make([]xealth_client.ObservationComponent, 0, len(stats))
	for _, s := range stats {
		component := xealth_client.ObservationComponent{
			Code: codeableConcept(s.Code, s.Display),
		}

		// Statistics are already formatted by the ehr package; Kind says how
		// to interpret the value string for the FHIR value[x] field.
		switch s.Kind {
		case ehr.KindDate:
			dt := s.Value
			component.ValueDateTime = &dt
		case ehr.KindInteger:
			i, err := strconv.Atoi(s.Value)
			if err != nil {
				continue
			}
			if s.Unit != "" {
				// valueInteger cannot carry a unit; use a quantity so the
				// day/hour units survive (parity with the Redox flowsheet).
				component.ValueQuantity = quantityFor(float64(i), s.Unit)
			} else {
				component.ValueInteger = &i
			}
		case ehr.KindDecimal:
			v, err := strconv.ParseFloat(s.Value, 64)
			if err != nil {
				continue
			}
			component.ValueQuantity = quantityFor(v, s.Unit)
		default:
			continue
		}

		components = append(components, component)
	}
	return components
}

func codeableConcept(code, display string) xealth_client.ObservationCodeableConcept {
	return xealth_client.ObservationCodeableConcept{
		Coding: &[]xealth_client.ObservationCoding{{
			System:  TidepoolObservationSystem,
			Code:    code,
			Display: ptr(display),
		}},
	}
}

// quantityFor maps a float statistic to a FHIR Quantity. Percentages and
// glucose values carry their display unit; unitless metrics (GMI, coefficient
// of variation, average daily records) carry only a value.
func quantityFor(value float64, unit string) *xealth_client.ObservationQuantity {
	q := &xealth_client.ObservationQuantity{Value: value}
	if unit != "" {
		q.Unit = ptr(unit)
	}
	return q
}

func ptr[T any](v T) *T { return &v }
