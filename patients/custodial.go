package patients

import (
	"context"
	"errors"
	"fmt"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type CustodialService interface {
	CreateAccount(ctx context.Context, patient Patient) (string, error)
	UpdateAccount(ctx context.Context, patient Patient) error
}

type custodialService struct {
	patientsRepo Repository
	userService  UserService
	logger       *zap.SugaredLogger
}

type CustodialServiceParams struct {
	fx.In

	PatientsRepo Repository
	UserService  UserService
	Logger       *zap.SugaredLogger
}

func NewCustodialService(p CustodialServiceParams) (CustodialService, error) {
	return &custodialService{
		patientsRepo: p.PatientsRepo,
		userService:  p.UserService,
		logger:       p.Logger,
	}, nil
}

func (c *custodialService) CreateAccount(ctx context.Context, patient Patient) (string, error) {
	c.logger.Debugw("creating custodial user", "patient", patient)
	user, err := c.userService.CreateCustodialAccount(ctx, patient)
	if errors.Is(err, shoreline.ErrDuplicateUser) {
		return "", ErrDuplicateEmail
	} else if err != nil {
		return "", fmt.Errorf("unable to create custodial user: %w", err)
	} else if user.UserID == "" {
		return "", fmt.Errorf("unexpected empty user id for custodial user")
	}

	return user.UserID, nil
}

func (c *custodialService) UpdateAccount(ctx context.Context, patient Patient) error {
	c.logger.Debugw("updating custodial user", zap.String("userId", *patient.UserId))
	if err := c.userService.UpdateCustodialAccount(ctx, patient); err != nil {
		return fmt.Errorf("unable to update custodial user: %w", err)
	}
	return nil
}
