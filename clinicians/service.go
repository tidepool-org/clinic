package clinicians

import (
	"context"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type service struct {
	dbClient       *mongo.Client
	clinicsService clinics.Service
	repository     *Repository
	logger         *zap.SugaredLogger
	userService    patients.UserService
}

var _ Service = &service{}

func NewService(dbClient *mongo.Client, clinicsService clinics.Service, repository *Repository, logger *zap.SugaredLogger, userService patients.UserService) (Service, error) {
	return &service{
		dbClient:       dbClient,
		clinicsService: clinicsService,
		repository:     repository,
		logger:         logger,
		userService:    userService,
	}, nil
}

func (s *service) Create(ctx context.Context, clinician *Clinician) (*Clinician, error) {
	result, err := store.WithTransaction(ctx, s.dbClient, func(sessionCtx mongo.SessionContext) (interface{}, error) {
		created, err := s.repository.Create(ctx, clinician)
		if err != nil {
			return nil, err
		}

		if err := s.onUpdate(ctx, clinician); err != nil {
			return nil, err
		}

		return created, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*Clinician), nil
}

func (s service) Update(ctx context.Context, update *ClinicianUpdate) (*Clinician, error) {
	result, err := store.WithTransaction(ctx, s.dbClient, func(sessionCtx mongo.SessionContext) (interface{}, error) {
		if err := s.onUpdate(ctx, &update.Clinician); err != nil {
			return nil, err
		}

		updated, err := s.repository.Update(sessionCtx, update)
		if err != nil {
			return nil, err
		}

		return updated, err
	})

	if err != nil {
		return nil, err
	}

	return result.(*Clinician), nil
}

func (s service) AssociateInvite(ctx context.Context, associate AssociateInvite) (*Clinician, error) {
	profile, err := s.userService.GetUserProfile(ctx, associate.UserId)
	if err != nil {
		return nil, err
	}
	associate.ClinicianName = profile.FullName

	result, err := store.WithTransaction(ctx, s.dbClient, func(sessionCtx mongo.SessionContext) (interface{}, error) {
		// Associate invite clinician record to the user id
		clinician, err := s.repository.AssociateInvite(sessionCtx, associate)
		if err != nil {
			return nil, err
		}

		if err := s.onUpdate(ctx, clinician); err != nil {
			return nil, err
		}

		return clinician, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*Clinician), nil
}

func (s *service) Get(ctx context.Context, clinicId string, clinicianId string) (*Clinician, error) {
	return s.repository.Get(ctx, clinicId, clinicianId)
}

func (s *service) List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinician, error) {
	return s.repository.List(ctx, filter, pagination)
}

func (s *service) Delete(ctx context.Context, clinicId string, clinicianId string) error {
	_, err := store.WithTransaction(ctx, s.dbClient, func(sessionCtx mongo.SessionContext) (interface{}, error) {
		clinician, err := s.repository.Get(ctx, clinicId, clinicianId)
		if err != nil {
			return nil, err
		}

		err = s.repository.Delete(ctx, clinicId, clinicianId)
		if err != nil {
			return nil, err
		}

		// Make sure the clinician is removed from the clinic record
		clinician.Roles = nil
		err = s.onUpdate(ctx, clinician)
		return nil, err
	})

	return err
}

// onUpdate makes sure the clinic object "admins" attribute is consistent with the admins in the clinicians collection.
// It must be executed on every operation that can change the roles of clinician.
func (s *service) onUpdate(ctx context.Context, clinician *Clinician) error {
	if clinician.UserId == nil {
		return nil
	}

	if clinician.IsAdmin() {
		// Make sure clinician user id is admin in clinic record
		return s.clinicsService.UpsertAdmin(ctx, clinician.ClinicId.Hex(), *clinician.UserId)
	}

	// Make sure clinician user id is removed as an admin from a clinic record
	return s.clinicsService.RemoveAdmin(ctx, clinician.ClinicId.Hex(), *clinician.UserId)
}

func (s *service) GetInvite(ctx context.Context, clinicId, inviteId string) (*Clinician, error) {
	return s.repository.GetInvite(ctx, clinicId, inviteId)
}

func (s *service) DeleteInvite(ctx context.Context, clinicId, inviteId string) error {
	return s.repository.DeleteInvite(ctx, clinicId, inviteId)
}
