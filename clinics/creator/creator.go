package creator

import (
	"context"
	"fmt"

	"github.com/kelseyhightower/envconfig"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
)

const (
	duplicateShareCodeRetryAttempts = 100
)

type Config struct {
	ClinicDemoPatientUserId string `envconfig:"CLINIC_DEMO_PATIENT_USER_ID"`
}

func NewConfig() (*Config, error) {
	c := &Config{}
	err := envconfig.Process("", c)
	return c, err
}

type CreateClinic struct {
	Clinic            clinics.Clinic
	CreatorUserId     string
	CreateDemoPatient bool
}

type Creator interface {
	CreateClinic(ctx context.Context, create *CreateClinic) (*clinics.Clinic, error)
}

type creator struct {
	clinics              clinics.Service
	cliniciansRepository *clinicians.Repository
	config               *Config
	dbClient             *mongo.Client
	patientsService      patients.Service
	shareCodeGenerator   clinics.ShareCodeGenerator
	userService          patients.UserService
}

type Params struct {
	fx.In

	Clinics              clinics.Service
	CliniciansRepository *clinicians.Repository
	Config               *Config
	DbClient             *mongo.Client
	PatientsService      patients.Service
	ShareCodeGenerator   clinics.ShareCodeGenerator
	UserService          patients.UserService
}

func NewCreator(cp Params) (Creator, error) {
	return &creator{
		clinics:              cp.Clinics,
		cliniciansRepository: cp.CliniciansRepository,
		config:               cp.Config,
		dbClient:             cp.DbClient,
		patientsService:      cp.PatientsService,
		shareCodeGenerator:   cp.ShareCodeGenerator,
		userService:          cp.UserService,
	}, nil
}

func (c *creator) CreateClinic(ctx context.Context, create *CreateClinic) (*clinics.Clinic, error) {
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

		// Set new clinic migration status to true.
		// Only clinics created via `EnableNewClinicExperience` handler should be subject to initial clinician patient migration
		create.Clinic.IsMigrated = true

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

// Creates a clinic document in mongo and retries if there is a violation of the unique share code constraint
func (c *creator) createClinicObject(sessionCtx mongo.SessionContext, create *CreateClinic) (clinic *clinics.Clinic, err error) {
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

func (c *creator) getDemoPatient(ctx context.Context) (*patients.Patient, error) {
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
