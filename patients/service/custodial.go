package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type CustodialService interface {
	CreateAccount(ctx context.Context, patient patients.Patient) (string, error)
	UpdateAccount(ctx context.Context, patient patients.Patient) error
	DeleteAccount(ctx context.Context, clinicID, userID string, metadata deletions.Metadata) error
}

type custodialService struct {
	patientsRepo patients.Repository
	userService  patients.UserService
	logger       *zap.SugaredLogger
	dataClient   clients.DataClient
}

type CustodialServiceParams struct {
	fx.In

	PatientsRepo patients.Repository
	UserService  patients.UserService
	Logger       *zap.SugaredLogger
	Data         clients.DataClient
}

func NewCustodialService(p CustodialServiceParams) (CustodialService, error) {
	return &custodialService{
		patientsRepo: p.PatientsRepo,
		userService:  p.UserService,
		logger:       p.Logger,
		dataClient:   p.Data,
	}, nil
}

func (c *custodialService) CreateAccount(ctx context.Context, patient patients.Patient) (string, error) {
	c.logger.Debugw("creating custodial user", "patient", patient)
	user, err := c.userService.CreateCustodialAccount(ctx, patient)
	if errors.Is(err, shoreline.ErrDuplicateUser) {
		return "", patients.ErrDuplicateEmail
	} else if err != nil {
		return "", fmt.Errorf("unable to create custodial user: %w", err)
	} else if user.UserID == "" {
		return "", fmt.Errorf("unexpected empty user id for custodial user")
	}

	return user.UserID, nil
}

func (c *custodialService) UpdateAccount(ctx context.Context, patient patients.Patient) error {
	c.logger.Debugw("updating custodial user", zap.String("userId", *patient.UserId))
	if err := c.userService.UpdateCustodialAccount(ctx, patient); err != nil {
		return fmt.Errorf("unable to update custodial user: %w", err)
	}
	return nil
}

func (c *custodialService) DeleteAccount(ctx context.Context, clinicID, userID string, metadata deletions.Metadata) error {
	c.logger.Debugw("Deleting custodial patient's user account", zap.String("userId", userID))
	hasData, err := c.dataClient.HasAnyData(userID)
	if err != nil {
		return err
	}
	if hasData {
		emptyEmail := ""
		if err := c.patientsRepo.UpdateEmail(ctx, userID, &emptyEmail); err != nil {
			return fmt.Errorf("unable to update custodial patients's email: %w", err)
		}
	} else {
		if err := c.patientsRepo.Remove(ctx, clinicID, userID, metadata); err != nil {
			return fmt.Errorf("unable remove custodial patient: %w", err)
		}
	}
	if err := c.userService.DeleteUserAccount(ctx, userID); err != nil {
		return fmt.Errorf("unable to delete custodial patients's user account: %w", err)
	}
	return nil
}
