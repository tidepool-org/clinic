package api

import (
	"github.com/tidepool-org/clinic/auth"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	"github.com/tidepool-org/clinic/clinics/merge"
	"github.com/tidepool-org/clinic/clinics/migration"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/redox"
	"github.com/tidepool-org/clinic/store"
	"github.com/tidepool-org/clinic/xealth"
	"go.uber.org/fx"
)

type Handler struct {
	fx.In

	ClinicMergePlanExecutor     merge.ClinicPlanExecutor
	Clinics                     clinics.Service
	ClinicsManager              manager.Manager
	ClinicsMigrator             migration.Migrator
	Clinicians                  clinicians.Service
	Patients                    patients.Service
	Redox                       redox.Redox
	Xealth                      xealth.Xealth
	ServiceAccountAuthenticator *auth.ServiceAccountAuthenticator
	Users                       patients.UserService
}

var _ ServerInterface = &Handler{}

func pagination(offset *Offset, limit *Limit) store.Pagination {
	page := store.DefaultPagination()
	if offset != nil {
		page.Offset = *offset
	}
	if limit != nil {
		page.Limit = *limit
	}
	return page
}
