package api

import (
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/store"
	"github.com/tidepool-org/clinic/summary"
	"net/http"
)

var periods = []string{"1d", "7d", "14d", "30d"}

func (h *Handler) ListCGMSummaries(ec echo.Context, clinicId ClinicId, params ListPatientSummariesParams) error {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)
	filter := summary.Filter{
		ClinicId:           strp(string(clinicId)),
		Search:             searchToString(params.Search),
		LastUploadDateFrom: params.DatesLastUploadDateFrom,
		LastUploadDateTo:   params.DatesLastUploadDateTo,
	}

	var sorts []*store.Sort

	if params.StatsTimeCGMUsePercent != nil && *params.StatsTimeCGMUsePercent != "" {
		cmp, value, err := parseRangeFilter(*params.StatsTimeCGMUsePercent)
		if err != nil {
			return err
		}
		filter.TimeCGMUsePercentCmp = cmp
		filter.TimeCGMUsePercentValue = value
	}
	if params.StatsTimeInVeryLowPercent != nil && *params.StatsTimeInVeryLowPercent != "" {
		cmp, value, err := parseRangeFilter(*params.StatsTimeInVeryLowPercent)
		if err != nil {
			return err
		}
		filter.TimeInVeryLowPercentCmp = cmp
		filter.TimeInVeryLowPercentValue = value
	}
	if params.StatsTimeInLowPercent != nil && *params.StatsTimeInLowPercent != "" {
		cmp, value, err := parseRangeFilter(*params.StatsTimeInLowPercent)
		if err != nil {
			return err
		}
		filter.TimeInLowPercentCmp = cmp
		filter.TimeInLowPercentValue = value
	}
	if params.StatsTimeInTargetPercent != nil && *params.StatsTimeInTargetPercent != "" {
		cmp, value, err := parseRangeFilter(*params.StatsTimeInTargetPercent)
		if err != nil {
			return err
		}
		filter.TimeInTargetPercentCmp = cmp
		filter.TimeInTargetPercentValue = value
	}
	if params.StatsTimeInHighPercent != nil && *params.StatsTimeInHighPercent != "" {
		cmp, value, err := parseRangeFilter(*params.StatsTimeInHighPercent)
		if err != nil {
			return err
		}
		filter.TimeInHighPercentCmp = cmp
		filter.TimeInHighPercentValue = value
	}
	if params.StatsTimeInVeryHighPercent != nil && *params.StatsTimeInVeryHighPercent != "" {
		cmp, value, err := parseRangeFilter(*params.StatsTimeInVeryHighPercent)
		if err != nil {
			return err
		}
		filter.TimeInVeryHighPercentCmp = cmp
		filter.TimeInVeryHighPercentValue = value
	}

	sorts, err := ParseSort(params.Sort)
	if err != nil {
		return err
	}

	list, err := h.cgmsummaries.List(ctx, &filter, page, sorts)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientSummariesResponseDto(list))
}

func (h *Handler) UpdatePatientCGMSummary(ec echo.Context) error {
	ctx := ec.Request().Context()
	var dto *PatientSummary
	if ec.Request().ContentLength != 0 {
		dto = &PatientSummary{}
		if err := ec.Bind(dto); err != nil {
			return err
		}
	}

	for _, period := range periods {
		s, err := NewSummary[summary.CGMPeriod](dto, period)
		err = h.cgmsummaries.CreateOrUpdate(ctx, s)
		if err != nil {
			return err
		}
	}

	return ec.NoContent(http.StatusOK)
}
