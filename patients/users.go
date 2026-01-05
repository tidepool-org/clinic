package patients

import (
	"context"
	"errors"
	"net/http"

	"go.uber.org/fx"

	clinicErrs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/platform/auth"
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

//go:generate go tool mockgen -source=./users.go -destination=./test/mock_users.go -package test

type UserService interface {
	CreateCustodialAccount(ctx context.Context, patient Patient) (*shoreline.UserData, error)
	DeleteCustodialAccount(ctx context.Context, userId string) error
	GetUser(userId string) (*shoreline.UserData, error)
	GetUserProfile(ctx context.Context, userId string) (*Profile, error)
	UpdateCustodialAccount(ctx context.Context, patient Patient) error
	PopulatePatientDetailsFromExistingUser(ctx context.Context, patient *Patient) error
}

type userService struct {
	shorelineClient shoreline.Client
	seagull         clients.Seagull
	gatekeeper      clients.Gatekeeper
	data            clients.DataClient
	authClient      auth.Client
}

var _ UserService = &userService{}

type UserServiceParams struct {
	fx.In

	ShorelineClient shoreline.Client
	Seagull         clients.Seagull
	Gatekeeper      clients.Gatekeeper
	Data            clients.DataClient
	Auth            auth.Client
}

func NewUserService(p UserServiceParams) (UserService, error) {
	return &userService{
		shorelineClient: p.ShorelineClient,
		seagull:         p.Seagull,
		gatekeeper:      p.Gatekeeper,
		data:            p.Data,
		authClient:      p.Auth,
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
		return errors.New("user id is missing")
	}

	emails := make([]string, 0)
	if patient.Email != nil {
		emails = append(emails, *patient.Email)
	}

	err := s.shorelineClient.UpdateUser(*patient.UserId, shoreline.UserUpdate{
		Username: patient.Email,
		Emails:   &emails,
	}, s.shorelineClient.TokenProvide())
	var statusErr *status.StatusError
	if errors.As(err, &statusErr) && statusErr.Code == http.StatusConflict {
		return ErrDuplicateEmail
	}
	return err
}

func (s *userService) GetUser(userId string) (*shoreline.UserData, error) {
	user, err := s.shorelineClient.GetUser(userId, s.shorelineClient.TokenProvide())
	if err != nil {
		var e *status.StatusError
		if errors.As(err, &e) && e.Code == http.StatusNotFound {
			return nil, clinicErrs.NotFound
		}
		return nil, err
	}
	return user, nil

}

func (s *userService) DeleteCustodialAccount(ctx context.Context, userId string) error {
	if err := s.authClient.DeleteAllRestrictedTokens(ctx, userId); err != nil {
		return err
	}
	hasData, err := s.data.HasAnyData(userId)
	if err != nil {
		return err
	}
	if !hasData {
		// Only custodial users with NO data can have their user account actually deleted.
		if err := s.shorelineClient.DeleteUser(userId, s.shorelineClient.TokenProvide()); err != nil {
			var e *status.StatusError
			if errors.As(err, &e) && e.Code == http.StatusNotFound {
				return clinicErrs.NotFound
			}
			return err
		}
	} else {
		// Otherwise, users with data will have their email address removed from their account, but the keycloak user won't actually be deleted.
		emptyUsername := ""
		emptyEmails := []string{""}
		if err := s.shorelineClient.UpdateUser(userId, shoreline.UserUpdate{
			Username: &emptyUsername,
			Emails:   &emptyEmails,
		}, s.shorelineClient.TokenProvide()); err != nil {
			return err
		}
	}
	return nil
}

func (s *userService) PopulatePatientDetailsFromExistingUser(ctx context.Context, patient *Patient) error {
	user, err := s.GetUser(*patient.UserId)
	if err != nil {
		return err
	}

	profile, err := s.GetUserProfile(ctx, *patient.UserId)
	if err != nil {
		return err
	}

	PopulatePatientFromUserAndProfile(patient, *user, *profile)

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

func PopulatePatientFromUserAndProfile(patient *Patient, user shoreline.UserData, profile Profile) {
	if patient.BirthDate == nil || *patient.BirthDate == "" {
		patient.BirthDate = profile.Patient.Birthday
	}
	if patient.Mrn == nil || *patient.Mrn == "" {
		patient.Mrn = profile.Patient.Mrn
	}
	if patient.FullName == nil || *patient.FullName == "" {
		patient.FullName = profile.Patient.FullName
	}

	patient.TargetDevices = profile.Patient.TargetDevices
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
}
