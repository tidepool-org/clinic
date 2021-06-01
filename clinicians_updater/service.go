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
}

var _ Service = &service{}

func NewService(dbClient *mongo.Client, clinicsService clinics.Service, cliniciansService clinicians.Service) (Service, error) {
	return &service{
		dbClient:          dbClient,
		clinicsService:    clinicsService,
		cliniciansService: cliniciansService,
	}, nil
}

func (s service) Update(ctx context.Context, clinicId string, clinicianId string, clinician *clinicians.Clinician) (*clinicians.Clinician, error) {
	session, err := s.dbClient.StartSession()
	if err != nil {
		return nil, fmt.Errorf("unable to start sessions %w", err)
	}
	defer session.EndSession(ctx)

	transaction := func(sessionCtx mongo.SessionContext) (interface{}, error) {
		if clinician.IsAdmin() {
			if err := s.clinicsService.UpsertAdmin(sessionCtx, clinicId, clinicianId); err != nil {
				return nil, err
			}
		} else {
			if err := s.clinicsService.RemoveAdmin(sessionCtx, clinicId, clinicianId); err != nil {
				return nil, err
			}
		}
		return s.cliniciansService.Update(sessionCtx, clinicId, clinicianId, clinician)
	}

	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)
	result, err := session.WithTransaction(ctx, transaction, txnOpts)
	if err != nil {
		return nil, err
	}

	return result.(*clinicians.Clinician), nil
}

func (s service) AssociateInvite(ctx context.Context, clinicId, inviteId, userId string) (*clinicians.Clinician, error) {
	session, err := s.dbClient.StartSession()
	if err != nil {
		return nil, fmt.Errorf("unable to start sessions %w", err)
	}
	defer session.EndSession(ctx)

	transaction := func(sessionCtx mongo.SessionContext) (interface{}, error) {
		clinician, err := s.cliniciansService.AssociateInvite(sessionCtx, clinicId, inviteId, userId)
		if err != nil {
			return nil, err
		}

		if clinician.IsAdmin() {
			if err := s.clinicsService.UpsertAdmin(sessionCtx, clinicId, *clinician.UserId); err != nil {
				return nil, err
			}
		} else {
			if err := s.clinicsService.RemoveAdmin(sessionCtx, clinicId, *clinician.UserId); err != nil {
				return nil, err
			}
		}

		return clinician, nil
	}

	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)
	result, err := session.WithTransaction(ctx, transaction, txnOpts)
	if err != nil {
		return nil, err
	}

	return result.(*clinicians.Clinician), nil
}
