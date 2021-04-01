package clinics

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/clinicians"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.uber.org/fx"
)

type CreateClinic struct {
	Clinic        Clinic
	CreatorUserId string
}

type Creator interface {
	CreateClinic(ctx context.Context, create *CreateClinic) (*Clinic, error)
}

type creator struct {
	clinics           Service
	cliniciansService clinicians.Service
	dbClient          *mongo.Client
}

type CreatorParams struct {
	fx.In

	Clinics           Service
	CliniciansService clinicians.Service
	DbClient          *mongo.Client
}

func NewCreator(cp CreatorParams) (Creator, error) {
	return &creator{
		clinics:           cp.Clinics,
		cliniciansService: cp.CliniciansService,
		dbClient:          cp.DbClient,
	}, nil
}

func (c *creator) CreateClinic(ctx context.Context, create *CreateClinic) (*Clinic, error) {
	session, err := c.dbClient.StartSession()
	if err != nil {
		return nil, fmt.Errorf("unable to start sessions %w", err)
	}
	defer session.EndSession(ctx)

	transaction := func(sessionCtx mongo.SessionContext) (interface{}, error) {
		clinic, err := c.clinics.Create(sessionCtx, &create.Clinic)
		if err != nil {
			return nil, err
		}

		clinician := &clinicians.Clinician{
			ClinicId: clinic.Id,
			UserId:   &create.CreatorUserId,
			Roles:    []string{clinicians.ClinicAdmin},
		}
		if _, err = c.cliniciansService.Create(sessionCtx, clinician); err != nil {
			return nil, err
		}

		return clinic, nil
	}

	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)
	result, err := session.WithTransaction(ctx, transaction, txnOpts)
	if err != nil {
		return nil, err
	}

	return result.(*Clinic), nil
}
