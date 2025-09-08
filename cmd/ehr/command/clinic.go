package command

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/store"
)

func getEHRClinic(clinicId string, clinicsService clinics.Service) (*clinics.Clinic, error) {
	enabled := true
	page := store.DefaultPagination().WithLimit(1)
	list, err := clinicsService.List(context.TODO(), &clinics.Filter{
		Ids:        []string{clinicId},
		EHREnabled: &enabled,
	}, page)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("the clinic does not have an active EHR integration")
	}

	return list[0], nil
}
