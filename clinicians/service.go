package clinicians

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.uber.org/zap"
)

type service struct {
	dbClient       *mongo.Client
	clinicsService clinics.Service
	repository     *Repository
	logger         *zap.SugaredLogger
}

var _ Service = &service{}

func NewService(dbClient *mongo.Client, clinicsService clinics.Service, repository *Repository, logger *zap.SugaredLogger) (Service, error) {
	return &service{
		dbClient:       dbClient,
		clinicsService: clinicsService,
		repository:     repository,
		logger:         logger,
	}, nil
}

func (s *service) Create(ctx context.Context, clinician *Clinician) (*Clinician, error) {
	session, err := s.dbClient.StartSession()
	if err != nil {
		return nil, fmt.Errorf("unable to start sessions %w", err)
	}
	defer session.EndSession(ctx)

	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)
	if err = session.StartTransaction(txnOpts); err != nil {
		return nil, err
	}

	var result *Clinician
	err = mongo.WithSession(ctx, session, func(sessionCtx mongo.SessionContext) error {
		created, err := s.repository.Create(ctx, clinician)
		if err != nil {
			if txnErr := session.AbortTransaction(sessionCtx); txnErr != nil {
				s.logger.Error("error when aborting transaction", zap.Error(txnErr))
			}
			return err
		}

		if created.UserId != nil {
			if created.IsAdmin() {
				if err := s.clinicsService.UpsertAdmin(sessionCtx, created.ClinicId.Hex(), *created.UserId); err != nil {
					if txnErr := session.AbortTransaction(sessionCtx); txnErr != nil {
						s.logger.Error("error when aborting transaction", zap.Error(txnErr))
					}
					return err
				}
			} else {
				if err := s.clinicsService.RemoveAdmin(sessionCtx, created.ClinicId.Hex(), *created.UserId); err != nil {
					if txnErr := session.AbortTransaction(sessionCtx); txnErr != nil {
						s.logger.Error("error when aborting transaction", zap.Error(txnErr))
					}
					return err
				}
			}
		}

		err = session.CommitTransaction(sessionCtx)
		if err == nil {
			result = created
		}

		return err
	})

	return result, err
}

func (s service) Update(ctx context.Context, clinicId string, clinicianId string, clinician *Clinician) (*Clinician, error) {
	session, err := s.dbClient.StartSession()
	if err != nil {
		return nil, fmt.Errorf("unable to start sessions %w", err)
	}
	defer session.EndSession(ctx)

	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)
	if err = session.StartTransaction(txnOpts); err != nil {
		return nil, err
	}

	var result *Clinician
	err = mongo.WithSession(ctx, session, func(sessionCtx mongo.SessionContext) error {
		if clinician.IsAdmin() {
			if err := s.clinicsService.UpsertAdmin(sessionCtx, clinicId, clinicianId); err != nil {
				if txnErr := session.AbortTransaction(sessionCtx); txnErr != nil {
					s.logger.Error("error when aborting transaction", zap.Error(txnErr))
				}
				return err
			}
		} else {
			if err := s.clinicsService.RemoveAdmin(sessionCtx, clinicId, clinicianId); err != nil {
				if txnErr := session.AbortTransaction(sessionCtx); txnErr != nil {
					s.logger.Error("error when aborting transaction", zap.Error(txnErr))
				}
				return err
			}
		}
		updated, err := s.repository.Update(sessionCtx, clinicId, clinicianId, clinician)
		if err != nil {
			if txnErr := session.AbortTransaction(sessionCtx); txnErr != nil {
				s.logger.Error("error when aborting transaction", zap.Error(txnErr))
			}
			return err
		}

		err = session.CommitTransaction(sessionCtx)
		if err == nil {
			result = updated
		}

		return err
	})

	return result, err
}

func (s service) AssociateInvite(ctx context.Context, clinicId, inviteId, userId string) (*Clinician, error) {
	session, err := s.dbClient.StartSession()
	if err != nil {
		return nil, fmt.Errorf("unable to start sessions %w", err)
	}
	defer session.EndSession(ctx)

	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)
	if err = session.StartTransaction(txnOpts); err != nil {
		return nil, err
	}

	var result *Clinician
	err = mongo.WithSession(ctx, session, func(sessionCtx mongo.SessionContext) error {
		clinician, err := s.repository.AssociateInvite(sessionCtx, clinicId, inviteId, userId)
		if err != nil {
			if txnErr := session.AbortTransaction(sessionCtx); txnErr != nil {
				s.logger.Error("error when aborting transaction", zap.Error(txnErr))
			}
			return err
		}

		if clinician.IsAdmin() {
			if err := s.clinicsService.UpsertAdmin(sessionCtx, clinicId, *clinician.UserId); err != nil {
				if txnErr := session.AbortTransaction(sessionCtx); txnErr != nil {
					s.logger.Error("error when aborting transaction", zap.Error(txnErr))
				}
				return err
			}
		} else {
			if err := s.clinicsService.RemoveAdmin(sessionCtx, clinicId, *clinician.UserId); err != nil {
				if txnErr := session.AbortTransaction(sessionCtx); txnErr != nil {
					s.logger.Error("error when aborting transaction", zap.Error(txnErr))
				}
				return err
			}
		}

		err = session.CommitTransaction(sessionCtx)
		if err == nil {
			result = clinician
		}

		return err
	})

	return result, err
}

func (s *service) Get(ctx context.Context, clinicId string, clinicianId string) (*Clinician, error) {
	return s.repository.Get(ctx, clinicId, clinicianId)
}

func (s *service) List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinician, error) {
	return s.repository.List(ctx, filter, pagination)
}

func (s *service) Delete(ctx context.Context, clinicId string, clinicianId string) error {
	return s.repository.Delete(ctx, clinicId, clinicianId)
}

func (s *service) GetInvite(ctx context.Context, clinicId, inviteId string) (*Clinician, error) {
	return s.repository.GetInvite(ctx, clinicId, inviteId)
}

func (s *service) DeleteInvite(ctx context.Context, clinicId, inviteId string) error {
	return s.repository.DeleteInvite(ctx, clinicId, inviteId)
}
