package api

import (
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"github.com/tidepool-org/clinic/users"
	"go.uber.org/fx"
	"net/http"
)

type Handler struct {
	clinics    clinics.Service
	clinicians clinicians.Service
	patients   patients.Service
	users      users.Service
}

var _ ServerInterface = &Handler{}

type Params struct {
	fx.In

	Clinics    clinics.Service
	Clinicians clinicians.Service
	Patients   patients.Service
	Users      users.Service
}

func NewHandler(p Params) *Handler {
	return &Handler{
		clinics:    p.Clinics,
		clinicians: p.Clinicians,
		patients:   p.Patients,
		users:      p.Users,
	}
}

func pagination(offset, limit *int) store.Pagination {
	page := store.DefaultPagination()
	if offset != nil {
		page.Offset = *offset
	}
	if limit != nil {
		page.Limit = *limit
	}
	return page
}
