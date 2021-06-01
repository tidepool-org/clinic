package clinics

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.uber.org/fx"
)

const (
	duplicateShareCodeRetryAttempts = 100
)

type CreateClinic struct {
	Clinic        Clinic
	CreatorUserId string
}

type Creator interface {
	CreateClinic(ctx context.Context, create *CreateClinic) (*Clinic, error)
}

type creator struct {
	clinics            Service
	cliniciansService  clinicians.Service
	dbClient           *mongo.Client
	shareCodeGenerator ShareCodeGenerator
	userService        shoreline.Client
}

type CreatorParams struct {
	fx.In

	Clinics            Service
	CliniciansService  clinicians.Service
	DbClient           *mongo.Client
	ShareCodeGenerator ShareCodeGenerator
	UserService        shoreline.Client
}

func NewCreator(cp CreatorParams) (Creator, error) {
	return &creator{
		clinics:            cp.Clinics,
		cliniciansService:  cp.CliniciansService,
		dbClient:           cp.DbClient,
		shareCodeGenerator: cp.ShareCodeGenerator,
		userService:        cp.UserService,
	}, nil
}

func (c *creator) CreateClinic(ctx context.Context, create *CreateClinic) (*Clinic, error) {
	user, err := c.userService.GetUser(create.CreatorUserId, c.userService.TokenProvide())
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("unable to find user with id %v", create.CreatorUserId)
	}

	session, err := c.dbClient.StartSession()
	if err != nil {
		return nil, fmt.Errorf("unable to start sessions %w", err)
	}
	defer session.EndSession(ctx)

	transaction := func(sessionCtx mongo.SessionContext) (interface{}, error) {
		// Set initial admins
		admins := []string{create.CreatorUserId}
		create.Clinic.Admins = &admins

		clinic, err := c.createClinicObject(sessionCtx, create)
		if err != nil {
			return nil, err
		}

		clinician := &clinicians.Clinician{
			ClinicId: clinic.Id,
			UserId:   &create.CreatorUserId,
			Roles:    []string{clinicians.ClinicAdmin},
			Email:    &user.Emails[0],
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

// Creates a clinic document in mongo and retries if there is a violation of the unique share code constraint
func (c *creator) createClinicObject(sessionCtx mongo.SessionContext, create *CreateClinic) (clinic *Clinic, err error) {
retryLoop:
	for i := 0; i < duplicateShareCodeRetryAttempts; i++ {
		shareCode := c.shareCodeGenerator.Generate()
		shareCodes := []string{shareCode}
		create.Clinic.CanonicalShareCode = &shareCode
		create.Clinic.ShareCodes = &shareCodes

		clinic, err = c.clinics.Create(sessionCtx, &create.Clinic)
		if err == nil || err != ErrDuplicateShareCode {
			break retryLoop
		}
	}
	return clinic, err
}

