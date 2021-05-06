package patients

import (
	"context"
	"fmt"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type CustodialService interface {
	CreateAccount(ctx context.Context, patient Patient) (*Patient, error)
	UpdateAccount(ctx context.Context, patient Patient) (*Patient, error)
}

type custodialService struct {
	patients    *Repository
	userService UserService
	logger      *zap.SugaredLogger
}

type CustodialServiceParams struct {
	fx.In

	PatientsRepo *Repository
	UserService  UserService
	Logger       *zap.SugaredLogger
}

func NewCustodialService(p CustodialServiceParams) (CustodialService, error) {
	return &custodialService{
		patients: p.PatientsRepo,
		userService: p.UserService,
		logger: p.Logger,
	}, nil
}

func (c *custodialService) CreateAccount(ctx context.Context, patient Patient) (*Patient, error) {
	c.logger.Debugw("creating custodial user", "patient", patient)
	user, err := c.userService.CreateCustodialAccount(ctx, patient)
	if err != nil {
		return nil, fmt.Errorf("unable to create custodial user: %w", err)
	}

	c.logger.Debugw("creating patient from custodial user", zap.String("userId", user.UserID))
	patient.UserId = &user.UserID
	clinicPatient, err := c.patients.Create(ctx, patient)
	if err != nil {
		return nil, fmt.Errorf("error creating patient from custodial user: %w", err)
	}

	return clinicPatient, nil
}

func (c *custodialService) UpdateAccount(ctx context.Context, patient Patient) (*Patient, error) {
	c.logger.Debugw("updating custodial user", zap.String("userId", *patient.UserId))
	if err := c.userService.UpdateCustodialAccount(ctx, patient); err != nil {
		return nil, fmt.Errorf("unable to update custodial user: %w", err)
	}

	c.logger.Debugw("updating custodial patient", zap.String("userId", *patient.UserId))
	clinicPatient, err := c.patients.Update(ctx, patient.ClinicId.Hex(), *patient.UserId, patient)
	if err != nil {
		return nil, fmt.Errorf("unable to update patient: %w", err)
	}

	return clinicPatient, nil
}
