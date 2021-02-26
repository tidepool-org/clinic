package users

import (
	"context"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	CreatePatientFromExistingUser(ctx context.Context, clinicId, userId string) (*patients.Patient, error)
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

func (s service) CreatePatientFromExistingUser(ctx context.Context, clinicId, userId string) (*patients.Patient, error) {
	patient, err := s.getPatientFromUser(clinicId, userId)
	if err != nil {
		return nil, err
	}

	return s.patients.Create(ctx, *patient)
}

func (s *service) getPatientFromUser(clinicId, userId string) (*patients.Patient, error) {
	user, err := s.getUser(userId)
	if err != nil {
		return nil, err
	}

	profile := Profile{}
	token := s.shorelineClient.TokenProvide()
	err = s.seagull.GetCollection(userId, "profile", token, &profile)
	if err != nil {
		return nil, err
	}
	
	//permissions, err := s.gatekeeper.UserInGroup(userId, clinicId)
	//if err != nil {
	//	return nil, err
	//}

	clinicObjId, err := primitive.ObjectIDFromHex(clinicId)
	if err != nil {
		return nil, err
	}

	email := profile.Patient.Email
	if email == nil {
		email = &user.Username
	}

	return &patients.Patient{
		UserId:        &userId,
		ClinicId:      &clinicObjId,
		Email:         email,
		BirthDate:     profile.Patient.Birthday,
		FullName:      profile.FullName,
		Mrn:           profile.Patient.Mrn,
		TargetDevices: profile.Patient.TargetDevices,
		//Permissions:   getPermissions(permissions),
	}, nil
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

func getPermissions(permissions clients.Permissions) *patients.PatientPermissions {
	return &patients.PatientPermissions{
		Upload: getPermission(permissions, "upload"),
		View:   getPermission(permissions, "view"),
		Note:   getPermission(permissions, "note"),
		Root:   getPermission(permissions, "root"),
	}
}

func getPermission(permissions clients.Permissions, permission string) *map[string]interface{} {
	if _, ok := permissions[permission]; !ok {
		return nil
	}
	p := make(map[string]interface{}, 0)
	return &p
}
