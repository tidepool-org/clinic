package patients

import (
	"context"
	"errors"
	"github.com/tidepool-org/clinic/store"
)

type service struct {
	repo *Repository
	custodialService CustodialService
}

var _ Service = &service{}

func NewService(repo *Repository, custodialService CustodialService) (Service, error) {
	return &service{
		repo:             repo,
		custodialService: custodialService,
	}, nil
}

func (s *service) Get(ctx context.Context, clinicId string, userId string) (*Patient, error) {
	return s.repo.Get(ctx, clinicId, userId)
}

func (s *service) List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Patient, error) {
	return s.repo.List(ctx, filter, pagination)
}

func (s *service) Create(ctx context.Context, patient Patient) (*Patient, error) {
	if patient.IsCustodial() {
		return s.custodialService.CreateAccount(ctx, patient)
	}
	if patient.UserId == nil {
		return nil, errors.New("user id is missing")
	}
	return s.repo.Create(ctx, patient)
}

func (s *service) Update(ctx context.Context, clinicId string, userId string, patient Patient) (*Patient, error) {
	existing, err := s.Get(ctx, clinicId, userId)
	if err != nil {
		return nil, err
	}

	if existing.IsCustodial() {
		patient.ClinicId = existing.ClinicId
		patient.UserId = existing.UserId
		return s.custodialService.UpdateAccount(ctx, patient)
	}
	return s.repo.Update(ctx, clinicId, userId, patient)
}

func (s *service) UpdatePermissions(ctx context.Context, clinicId, userId string, permissions *Permissions) (*Patient, error) {
	return s.repo.UpdatePermissions(ctx, clinicId, userId, permissions)
}

