package patients

import (
	"context"
	errs "errors"
	"net/http"

	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"go.uber.org/fx"
)

var UserServiceModule = fx.Provide(
	configProvider,
	httpClientProvider,
	shorelineProvider,
	gatekeeperProvider,
	seagullProvider,
	dataProvider,
	NewUserService,
)

//go:generate mockgen --build_flags=--mod=mod -source=./users.go -destination=./test/mock_users.go -package test MockUserService

type UserService interface {
	CreateCustodialAccount(ctx context.Context, patient Patient) (*shoreline.UserData, error)
	GetUser(userId string) (*shoreline.UserData, error)
	GetUserProfile(ctx context.Context, userId string) (*Profile, error)
	UpdateCustodialAccount(ctx context.Context, patient Patient) error
	GetPatientFromExistingUser(ctx context.Context, patient *Patient) error
}

type userService struct {
	shorelineClient shoreline.Client
	seagull         clients.Seagull
	gatekeeper      clients.Gatekeeper
	data            clients.DataClient
}

var _ UserService = &userService{}

type UserServiceParams struct {
	fx.In

	ShorelineClient shoreline.Client
	Seagull         clients.Seagull
	Gatekeeper      clients.Gatekeeper
	Data            clients.DataClient
}

func NewUserService(p UserServiceParams) (UserService, error) {
	return &userService{
		shorelineClient: p.ShorelineClient,
		seagull:         p.Seagull,
		gatekeeper:      p.Gatekeeper,
		data:            p.Data,
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
		Username: patient.Email,
		Emails:   &emails,
	}, s.shorelineClient.TokenProvide())
	if statusErr, ok := err.(*status.StatusError); ok && statusErr.Code == http.StatusConflict {
		return ErrDuplicateEmail
	}
	return err
}

func (s *userService) GetUser(userId string) (*shoreline.UserData, error) {
	user, err := s.shorelineClient.GetUser(userId, s.shorelineClient.TokenProvide())
	if err != nil {
		if e, ok := err.(*status.StatusError); ok && e.Code == http.StatusNotFound {
			return nil, errors.NotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *userService) GetPatientFromExistingUser(ctx context.Context, patient *Patient) error {
	user, err := s.GetUser(*patient.UserId)
	if err != nil {
		return err
	}

	profile, err := s.GetUserProfile(ctx, *patient.UserId)
	if err != nil {
		return err
	}

	patient.BirthDate = profile.Patient.Birthday
	patient.Mrn = profile.Patient.Mrn
	patient.TargetDevices = profile.Patient.TargetDevices
	patient.FullName = profile.Patient.FullName
	patient.Email = &user.Username

	if patient.FullName == nil || *patient.FullName == "" {
		patient.FullName = profile.FullName
	}
	if patient.Email != nil && *patient.Email == "" {
		patient.Email = nil
	}

	// Some profiles don't have birth dates
	// There isn't anything we can do, but to insert an empty string,
	// because the birth date is a required field.
	if patient.BirthDate == nil {
		birthDate := ""
		patient.BirthDate = &birthDate
	}

	sources, err := s.GetUserDataSources(ctx, *patient.UserId)
	if err != nil {
		return err
	}

	if len(sources) > 0 {
		var dataSources DataSources
		for _, source := range sources {
			dataSources = append(dataSources, DataSource{
				ModifiedTime: source.ModifiedTime,
				ProviderName: *source.ProviderName,
				State:        *source.State,
			})
		}

		patient.DataSources = (*[]DataSource)(&dataSources)
	}
	return nil
}

func (s *userService) GetUserProfile(ctx context.Context, userId string) (*Profile, error) {
	profile := Profile{}
	token := s.shorelineClient.TokenProvide()
	err := s.seagull.GetCollection(userId, "profile", token, &profile)
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (s *userService) GetUserDataSources(ctx context.Context, userId string) (clients.DataSourceArray, error) {
	sources, err := s.data.ListSources(string(userId))
	if err != nil {
		return nil, err
	}
	return sources, nil
}
