package api

import (
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"go.uber.org/fx"
)

type Handler struct {
	clinics        clinics.Service
	clinicsCreator clinics.Creator
	clinicians     clinicians.Service
	patients       patients.Service
	users          patients.UserService
}

var _ ServerInterface = &Handler{}

type Params struct {
	fx.In

	Clinics        clinics.Service
	ClinicsCreator clinics.Creator
	Clinicians     clinicians.Service
	Patients       patients.Service
	Users          patients.UserService
}

func NewHandler(p Params) *Handler {
	return &Handler{
		clinics:        p.Clinics,
		clinicsCreator: p.ClinicsCreator,
		clinicians:     p.Clinicians,
		patients:       p.Patients,
		users:          p.Users,
	}
}

func pagination(offset *Offset, limit *Limit) store.Pagination {
	page := store.DefaultPagination()
	if offset != nil {
		page.Offset = int(*offset)
	}
	if limit != nil {
		page.Limit = int(*limit)
	}
	return page
}
