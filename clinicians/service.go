package clinicians

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/deletions"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type service struct {
	dbClient        *mongo.Client
	clinicsService  clinics.Service
	repository      *Repository
	logger          *zap.SugaredLogger
	userService     patients.UserService
	patientsService patients.Service
}

var _ Service = &service{}

func NewService(dbClient *mongo.Client, clinicsService clinics.Service, repository *Repository, logger *zap.SugaredLogger, userService patients.UserService, patientsService patients.Service) (Service, error) {
	return &service{
		dbClient:        dbClient,
		clinicsService:  clinicsService,
		repository:      repository,
		logger:          logger,
		userService:     userService,
		patientsService: patientsService,
	}, nil
}

func (s *service) Create(ctx context.Context, clinician *Clinician) (*Clinician, error) {
	result, err := store.WithTransaction(ctx, s.dbClient, func(sessionCtx mongo.SessionContext) (interface{}, error) {
		created, err := s.repository.Create(sessionCtx, clinician)
		if err != nil {
			return nil, err
		}

		if created.UserId != nil {
			// When the user id is not set the clinician object is an invite
			// and is not yet associated with a user, thus the operation cannot
			// change clinic admins
			if err := s.onUpdate(sessionCtx, created, false); err != nil {
				return nil, err
			}
		}

		return created, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*Clinician), nil
}

func (s *service) Update(ctx context.Context, update *ClinicianUpdate) (*Clinician, error) {
	result, err := store.WithTransaction(ctx, s.dbClient, func(sessionCtx mongo.SessionContext) (interface{}, error) {
		updated, err := s.repository.Update(sessionCtx, update)
		if err != nil {
			return nil, err
		}

		if err := s.onUpdate(sessionCtx, updated, false); err != nil {
			return nil, err
		}

		return updated, err
	})

	if err != nil {
		return nil, err
	}

	return result.(*Clinician), nil
}

func (s *service) UpdateAll(ctx context.Context, update *CliniciansUpdate) error {
	return s.repository.UpdateAll(ctx, update)
}

func (s *service) AssociateInvite(ctx context.Context, associate AssociateInvite) (*Clinician, error) {
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

		if err := s.onUpdate(sessionCtx, clinician, false); err != nil {
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

func (s *service) Delete(ctx context.Context, clinicId string, clinicianId string, metadata deletions.Metadata) error {
	clinician, err := s.repository.Get(ctx, clinicId, clinicianId)
	if err != nil {
		return err
	}

	_, err = store.WithTransaction(ctx, s.dbClient, func(sessionCtx mongo.SessionContext) (interface{}, error) {
		return nil, s.deleteSingle(sessionCtx, clinician, metadata, false)
	})

	return err
}

func (s *service) DeleteAll(ctx context.Context, clinicId string, metadata deletions.Metadata) error {
	s.logger.Infow("deleting all clinicians", "clinicId", clinicId)
	_, err := store.WithTransaction(ctx, s.dbClient, func(sessCtx mongo.SessionContext) (interface{}, error) {
		return nil, s.repository.DeleteAll(ctx, clinicId, metadata)
	})
	return err
}

func (s *service) DeleteFromAllClinics(ctx context.Context, clinicianId string, metadata deletions.Metadata) error {
	_, err := store.WithTransaction(ctx, s.dbClient, func(sessCtx mongo.SessionContext) (interface{}, error) {
		filter := &Filter{
			UserId: &clinicianId,
		}
		pagination := store.Pagination{
			Offset: 0,
			Limit:  0, // Fetches all records from mongo
		}

		s.logger.Debugw("retrieving clinician records for user", "userId", clinicianId)
		clinicianList, err := s.List(sessCtx, filter, pagination)
		if err != nil {
			return nil, err
		}

		for _, clinician := range clinicianList {
			clinicId := clinician.ClinicId.Hex()
			if err := s.deleteSingle(sessCtx, clinician, metadata, true); err != nil {
				return nil, err
			}

			// Check if clinic has any remaining members
			filter = &Filter{
				ClinicId: &clinicId,
			}
			pagination = store.Pagination{
				Limit: 1,
			}
			remaining, err := s.List(sessCtx, filter, pagination)
			if err != nil {
				return nil, err
			}

			// Remove all connections to non-custodial accounts,
			// because the clinic doesn't have any clinicians
			if len(remaining) == 0 {
				s.logger.Infow("deleting all non-custodial patients of clinic", "clinicId", clinicId)
				if err = s.patientsService.DeleteNonCustodialPatientsOfClinic(sessCtx, clinicId, deletions.Metadata{}); err != nil {
					return nil, err
				}
			}

		}

		return nil, nil
	})

	return err
}

func (s *service) deleteSingle(ctx context.Context, clinician *Clinician, metadata deletions.Metadata, allowOrphaning bool) error {
	s.logger.Infow("deleting user from clinic", "userId", *clinician.UserId, "clinicId", clinician.ClinicId.Hex())
	err := s.repository.Delete(ctx, clinician.ClinicId.Hex(), *clinician.UserId, metadata)
	if err != nil {
		return err
	}

	// Make sure the clinician is removed from the clinic record
	clinician.Roles = nil
	return s.onUpdate(ctx, clinician, allowOrphaning)
}

// onUpdate makes sure the clinic object "admins" attribute is consistent with the admins in the clinicians collection.
// It must be executed on every operation that can change the roles of clinician.
func (s *service) onUpdate(ctx context.Context, updated *Clinician, allowOrphaning bool) error {
	if updated.UserId == nil {
		return fmt.Errorf("clinician user id cannot be empty")
	}

	if updated.IsAdmin() {
		// Make sure clinician user id is admin in clinic record
		return s.clinicsService.UpsertAdmin(ctx, updated.ClinicId.Hex(), *updated.UserId)
	}

	// Make sure clinician user id is removed as an admin from a clinic record
	err := s.clinicsService.RemoveAdmin(ctx, updated.ClinicId.Hex(), *updated.UserId, allowOrphaning)

	return err
}

func (s *service) GetInvite(ctx context.Context, clinicId, inviteId string) (*Clinician, error) {
	return s.repository.GetInvite(ctx, clinicId, inviteId)
}

func (s *service) DeleteInvite(ctx context.Context, clinicId, inviteId string) error {
	return s.repository.DeleteInvite(ctx, clinicId, inviteId)
}
