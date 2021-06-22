package patients

import (
	"context"
	"errors"
	"github.com/tidepool-org/clinic/store"
)

type service struct {
	repo Repository
	custodialService CustodialService
}

var _ Service = &service{}

func NewService(repo Repository, custodialService CustodialService) (Service, error) {
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

func (s *service) Remove(ctx context.Context, clinicId string, userId string) error {
	return s.repo.Remove(ctx, clinicId, userId)
}

func (s *service) UpdatePermissions(ctx context.Context, clinicId, userId string, permissions *Permissions) (*Patient, error) {
	if permissions == nil || permissions.Empty() {
		return nil, s.Remove(ctx, clinicId, userId)
	}
	return s.repo.UpdatePermissions(ctx, clinicId, userId, permissions)
}

func (s *service) DeletePermission(ctx context.Context, clinicId, userId, permission string) (*Patient, error) {
	patient, err := s.repo.DeletePermission(ctx, clinicId, userId, permission)
	if err != nil {
		return nil, err
	}
	if shouldRemovePatientFromClinic(patient) {
		if err := s.Remove(ctx, clinicId, userId); err != nil {
			// the patient was removed by concurrent request which is not a problem,
			// because it had to be removed as a result of the current operation
			if err == ErrNotFound {
				return nil, nil
			}
			return nil, err
		}
		return nil, nil
	}
	return patient, err
}

func shouldRemovePatientFromClinic(patient *Patient) bool {
	if patient != nil {
		return patient.Permissions == nil || patient.Permissions.Empty()
	}
	return false
}
