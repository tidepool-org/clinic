package manager

import (
	"context"
	errs "errors"
	"fmt"

	"github.com/tidepool-org/clinic/errors"

	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
)

const (
	duplicateShareCodeRetryAttempts = 100
)

type CreateClinic struct {
	Clinic            clinics.Clinic
	CreatorUserId     string
	CreateDemoPatient bool
}

type Manager interface {
	CreateClinic(ctx context.Context, create *CreateClinic) (*clinics.Clinic, error)
	DeleteClinic(ctx context.Context, clinicId string) error
	GetClinicPatientCount(ctx context.Context, clinicId string) (*clinics.PatientCount, error)
}

type manager struct {
	clinics              clinics.Service
	cliniciansRepository *clinicians.Repository
	config               *config.Config
	dbClient             *mongo.Client
	patientsService      patients.Service
	shareCodeGenerator   clinics.ShareCodeGenerator
	userService          patients.UserService
}

type Params struct {
	fx.In

	Clinics              clinics.Service
	CliniciansRepository *clinicians.Repository
	Config               *config.Config
	DbClient             *mongo.Client
	PatientsService      patients.Service
	ShareCodeGenerator   clinics.ShareCodeGenerator
	UserService          patients.UserService
}

func NewManager(cp Params) (Manager, error) {
	return &manager{
		clinics:              cp.Clinics,
		cliniciansRepository: cp.CliniciansRepository,
		config:               cp.Config,
		dbClient:             cp.DbClient,
		patientsService:      cp.PatientsService,
		shareCodeGenerator:   cp.ShareCodeGenerator,
		userService:          cp.UserService,
	}, nil
}

func (c *manager) CreateClinic(ctx context.Context, create *CreateClinic) (*clinics.Clinic, error) {
	user, err := c.userService.GetUser(create.CreatorUserId)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("unable to find user with id %v", create.CreatorUserId)
	}

	profile, err := c.userService.GetUserProfile(ctx, create.CreatorUserId)
	if err != nil {
		return nil, fmt.Errorf("error fetching user profile of clinician %v", create.CreatorUserId)
	}

	var demoPatient *patients.Patient
	if create.CreateDemoPatient {
		demoPatient, err = c.getDemoPatient(ctx)
		if err != nil {
			return nil, err
		}
	}

	transaction := func(sessionCtx mongo.SessionContext) (interface{}, error) {
		// Set initial admins
		create.Clinic.AddAdmin(create.CreatorUserId)

		// Add the clinic to the collection
		clinic, err := c.createClinicObject(sessionCtx, create)
		if err != nil {
			return nil, err
		}

		// Add the clinician to the collection
		clinician := &clinicians.Clinician{
			ClinicId: clinic.Id,
			UserId:   &create.CreatorUserId,
			Roles:    []string{clinicians.ClinicAdmin},
			Email:    &user.Emails[0],
		}
		if profile != nil {
			clinician.Name = profile.FullName
		}
		if _, err = c.cliniciansRepository.Create(sessionCtx, clinician); err != nil {
			return nil, err
		}

		// Add the demo patient account
		if demoPatient != nil {
			demoPatient.ClinicId = clinic.Id
			if _, err = c.patientsService.Create(sessionCtx, *demoPatient); err != nil {
				return nil, err
			}
		}

		return clinic, nil
	}

	result, err := store.WithTransaction(ctx, c.dbClient, transaction)
	if err != nil {
		return nil, err
	}

	return result.(*clinics.Clinic), nil
}

func (c *manager) DeleteClinic(ctx context.Context, clinicId string) error {
	transaction := func(sessionCtx mongo.SessionContext) (interface{}, error) {
		filter := patients.Filter{ClinicId: &clinicId}
		pagination := store.Pagination{Limit: 2}
		res, err := c.patientsService.List(sessionCtx, &filter, pagination, nil)

		if err != nil {
			return nil, err
		}
		if res == nil {
			return nil, fmt.Errorf("patient list result not defined")
		}
		if !c.patientListAllowsClinicDeletion(res.Patients) {
			return nil, fmt.Errorf("%w: deletion of non-empty clinics is not allowed", errors.BadRequest)
		}

		if err := c.patientsService.Remove(sessionCtx, clinicId, c.config.ClinicDemoPatientUserId); err != nil && !errs.Is(err, errors.NotFound) {
			return nil, err
		}

		if err := c.cliniciansRepository.DeleteAll(sessionCtx, clinicId); err != nil {
			return nil, err
		}

		return nil, c.clinics.Delete(sessionCtx, clinicId)
	}

	_, err := store.WithTransaction(ctx, c.dbClient, transaction)
	return err
}

func (c *manager) GetClinicPatientCount(ctx context.Context, clinicId string) (*clinics.PatientCount, error) {
	patientCount, err := c.clinics.GetPatientCount(ctx, clinicId)
	if err != nil {
		return nil, err
	}

	if patientCount == nil {
		count, err := c.patientsService.Count(ctx, &patients.Filter{ClinicId: &clinicId, ExcludeDemo: true})
		if err != nil {
			return nil, err
		}

		patientCount = &clinics.PatientCount{PatientCount: count}
		if err := c.clinics.UpdatePatientCount(ctx, clinicId, patientCount); err != nil {
			return nil, err
		}
	}

	return patientCount, nil
}

// Creates a clinic document in mongo and retries if there is a violation of the unique share code constraint
func (c *manager) createClinicObject(sessionCtx mongo.SessionContext, create *CreateClinic) (clinic *clinics.Clinic, err error) {
retryLoop:
	for i := 0; i < duplicateShareCodeRetryAttempts; i++ {
		shareCode := c.shareCodeGenerator.Generate()
		shareCodes := []string{shareCode}
		create.Clinic.CanonicalShareCode = &shareCode
		create.Clinic.ShareCodes = &shareCodes

		clinic, err = c.clinics.Create(sessionCtx, &create.Clinic)
		if err == nil || err != clinics.ErrDuplicateShareCode {
			break retryLoop
		}
	}
	return clinic, err
}

func (c *manager) getDemoPatient(ctx context.Context) (*patients.Patient, error) {
	if c.config.ClinicDemoPatientUserId == "" {
		return nil, nil
	}

	perm := make(patients.Permission, 0)
	patient := &patients.Patient{
		UserId:     &c.config.ClinicDemoPatientUserId,
		IsMigrated: true, // Do not send emails
		Permissions: &patients.Permissions{
			View: &perm,
		},
	}
	if err := c.userService.GetPatientFromExistingUser(ctx, patient); err != nil {
		return nil, err
	}
	return patient, nil
}

func (c *manager) patientListAllowsClinicDeletion(list []*patients.Patient) bool {
	// No patients, OK to delete
	if len(list) == 0 {
		return true
	}
	// Only demo patients, OK to delete
	if len(list) == 1 && list[0] != nil && list[0].UserId != nil && *list[0].UserId == c.config.ClinicDemoPatientUserId {
		return true
	}
	return false
}
