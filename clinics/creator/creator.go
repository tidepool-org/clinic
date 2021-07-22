package creator

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
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
	Clinic        clinics.Clinic
	CreatorUserId string
}

type Creator interface {
	CreateClinic(ctx context.Context, create *CreateClinic) (*clinics.Clinic, error)
}

type creator struct {
	clinics              clinics.Service
	cliniciansRepository *clinicians.Repository
	dbClient             *mongo.Client
	shareCodeGenerator   clinics.ShareCodeGenerator
	userService          shoreline.Client
}

type Params struct {
	fx.In

	Clinics              clinics.Service
	CliniciansRepository *clinicians.Repository
	DbClient             *mongo.Client
	ShareCodeGenerator   clinics.ShareCodeGenerator
	UserService          shoreline.Client
}

func NewCreator(cp Params) (Creator, error) {
	return &creator{
		clinics:              cp.Clinics,
		cliniciansRepository: cp.CliniciansRepository,
		dbClient:             cp.DbClient,
		shareCodeGenerator:   cp.ShareCodeGenerator,
		userService:          cp.UserService,
	}, nil
}

func (c *creator) CreateClinic(ctx context.Context, create *CreateClinic) (*clinics.Clinic, error) {
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
		if _, err = c.cliniciansRepository.Create(sessionCtx, clinician); err != nil {
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

	return result.(*clinics.Clinic), nil
}

// Creates a clinic document in mongo and retries if there is a violation of the unique share code constraint
func (c *creator) createClinicObject(sessionCtx mongo.SessionContext, create *CreateClinic) (clinic *clinics.Clinic, err error) {
retryLoop:
	for i := 0; i < duplicateShareCodeRetryAttempts; i++ {
		shareCode := c.shareCodeGenerator.Generate()
		shareCodes := []string{shareCode}
		create.Clinic.CanonicalShareCode = &shareCode
		create.Clinic.ShareCodes = &shareCodes

		clinic, err = c.clinics.Create(sessionCtx, &create.Clinic)
		if err == nil || err != clinics.ErrDuplicateShareCode {
			break retryLoop
		}
	}
	return clinic, err
}
