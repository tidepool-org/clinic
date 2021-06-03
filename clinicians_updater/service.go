package clinicians_updater

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.uber.org/zap"
)

// The service is used for updating clinicians and clinics in transactions
type Service interface {
	Update(ctx context.Context, clinicId string, id string, clinician *clinicians.Clinician) (*clinicians.Clinician, error)
	AssociateInvite(ctx context.Context, clinicId, inviteId, userId string) (*clinicians.Clinician, error)
}

type service struct {
	dbClient          *mongo.Client
	clinicsService    clinics.Service
	cliniciansService clinicians.Service
	logger            *zap.SugaredLogger
}

var _ Service = &service{}

func NewService(dbClient *mongo.Client, clinicsService clinics.Service, cliniciansService clinicians.Service, logger *zap.SugaredLogger) (Service, error) {
	return &service{
		dbClient:          dbClient,
		clinicsService:    clinicsService,
		cliniciansService: cliniciansService,
		logger:            logger,
	}, nil
}

func (s service) Update(ctx context.Context, clinicId string, clinicianId string, clinician *clinicians.Clinician) (*clinicians.Clinician, error) {
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

	var result *clinicians.Clinician
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
		updated, err := s.cliniciansService.Update(sessionCtx, clinicId, clinicianId, clinician)
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

func (s service) AssociateInvite(ctx context.Context, clinicId, inviteId, userId string) (*clinicians.Clinician, error) {
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

	var result *clinicians.Clinician
	err = mongo.WithSession(ctx, session, func(sessionCtx mongo.SessionContext) error {
		clinician, err := s.cliniciansService.AssociateInvite(sessionCtx, clinicId, inviteId, userId)
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
