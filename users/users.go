package users

import (
	"context"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"go.uber.org/fx"
	"net/http"
)

var Module = fx.Provide(
	configProvider,
	httpClientProvider,
	shorelineProvider,
	gatekeeperProvider,
	seagullProvider,
	NewService,
)

type Service interface {
	CreatePatientAccount(ctx context.Context, patient patients.Patient) (*patients.Patient, error)
	CreatePatientFromExistingUser(ctx context.Context, patient patients.Patient) (*patients.Patient, error)
}

type service struct {
	patients        patients.Service
	shorelineClient shoreline.Client
	seagull         clients.Seagull
	gatekeeper      clients.Gatekeeper
}

var _ Service = &service{}

type Params struct {
	fx.In

	Patients        patients.Service
	ShorelineClient shoreline.Client
	Seagull         clients.Seagull
	Gatekeeper      clients.Gatekeeper
}

func NewService(p Params) (Service, error) {
	return &service{
		patients:        p.Patients,
		shorelineClient: p.ShorelineClient,
		seagull:         p.Seagull,
		gatekeeper:      p.Gatekeeper,
	}, nil
}

func (s service) CreatePatientAccount(ctx context.Context, patient patients.Patient) (*patients.Patient, error) {
	panic("not implemented")
}

func (s service) CreatePatientFromExistingUser(ctx context.Context, patient patients.Patient) (*patients.Patient, error) {
	if err := s.updatePatientDetails(&patient); err != nil {
		return nil, err
	}

	return s.patients.Create(ctx, patient)
}

func (s *service) updatePatientDetails(patient *patients.Patient) error {
	user, err := s.getUser(*patient.UserId)
	if err != nil {
		return err
	}

	profile := Profile{}
	token := s.shorelineClient.TokenProvide()
	err = s.seagull.GetCollection(*patient.UserId, "profile", token, &profile)
	if err != nil {
		return err
	}

	patient.BirthDate = profile.Patient.Birthday
	patient.FullName = profile.Patient.Email
	patient.Mrn = profile.Patient.Mrn
	patient.TargetDevices = profile.Patient.TargetDevices
	patient.Email = profile.Patient.Email
	if patient.Email == nil || *patient.Email == "" {
		patient.Email = &user.Username
	}

	return nil
}

func (s *service) getUser(userId string) (*shoreline.UserData, error) {
	user, err := s.shorelineClient.GetUser(userId, s.shorelineClient.TokenProvide())
	if err != nil {
		if e, ok := err.(*status.StatusError); ok && e.Code == http.StatusNotFound {
			return nil, errors.NotFound
		}
		return nil, err
	}
	return user, nil
}
