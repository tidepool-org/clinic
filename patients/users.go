package patients

import (
	"context"
	errs "errors"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"go.uber.org/fx"
	"net/http"
)

var UserServiceModule = fx.Provide(
	configProvider,
	httpClientProvider,
	shorelineProvider,
	gatekeeperProvider,
	seagullProvider,
	NewUserService,
)

type UserService interface {
	CreateCustodialAccount(ctx context.Context, patient Patient) (*shoreline.UserData, error)
	UpdateCustodialAccount(ctx context.Context, patient Patient) error
	GetPatientFromExistingUser(ctx context.Context, patient *Patient) error
}

type userService struct {
	shorelineClient shoreline.Client
	seagull         clients.Seagull
	gatekeeper      clients.Gatekeeper
}

var _ UserService = &userService{}

type UserServiceParams struct {
	fx.In

	ShorelineClient shoreline.Client
	Seagull         clients.Seagull
	Gatekeeper      clients.Gatekeeper
}

func NewUserService(p UserServiceParams) (UserService, error) {
	return &userService{
		shorelineClient: p.ShorelineClient,
		seagull:         p.Seagull,
		gatekeeper:      p.Gatekeeper,
	}, nil
}

func (s *userService) CreateCustodialAccount(ctx context.Context, patient Patient) (*shoreline.UserData, error) {
	clinicId := patient.ClinicId.Hex()
	user := shoreline.CustodialUserData{
		Email: patient.Email,
	}
	return s.shorelineClient.CreateCustodialUserForClinic(clinicId, user, s.shorelineClient.TokenProvide())
}

func (s *userService) UpdateCustodialAccount(ctx context.Context, patient Patient) error {
	if patient.UserId == nil {
		return errs.New("user id is missing")
	}

	emails := make([]string, 0)
	if patient.Email != nil {
		emails = append(emails, *patient.Email)
	}

	err := s.shorelineClient.UpdateUser(*patient.UserId, shoreline.UserUpdate{
		Username:      patient.Email,
		Emails:        &emails,
	}, s.shorelineClient.TokenProvide())
	if statusErr, ok := err.(*status.StatusError); ok && statusErr.Code == http.StatusConflict {
		return ErrDuplicateEmail
	}
	return err
}

func (s *userService) GetPatientFromExistingUser(ctx context.Context, patient *Patient) error {
	return s.updatePatientDetails(patient)
}

func (s *userService) updatePatientDetails(patient *Patient) error {
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
	patient.Mrn = profile.Patient.Mrn
	patient.TargetDevices = profile.Patient.TargetDevices
	patient.FullName = profile.Patient.FullName
	if patient.FullName == nil || *patient.FullName == "" {
		patient.FullName = profile.FullName
	}
	patient.Email = profile.Patient.Email
	if patient.Email == nil || *patient.Email == "" {
		patient.Email = &user.Username
	}

	return nil
}

func (s *userService) getUser(userId string) (*shoreline.UserData, error) {
	user, err := s.shorelineClient.GetUser(userId, s.shorelineClient.TokenProvide())
	if err != nil {
		if e, ok := err.(*status.StatusError); ok && e.Code == http.StatusNotFound {
			return nil, errors.NotFound
		}
		return nil, err
	}
	return user, nil
}
