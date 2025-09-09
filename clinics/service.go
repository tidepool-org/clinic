package clinics

import (
	"context"

	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/store"
)

func NewService(repository Repository) (Service, error) {
	return &service{
		repository: repository,
	}, nil
}

type service struct {
	repository Repository
}

func (s *service) Get(ctx context.Context, id string) (*Clinic, error) {
	return s.repository.Get(ctx, id)
}

func (s *service) List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinic, error) {
	return s.repository.List(ctx, filter, pagination)
}

func (s *service) Create(ctx context.Context, clinic *Clinic) (*Clinic, error) {
	return s.repository.Create(ctx, clinic)
}

func (s *service) Update(ctx context.Context, id string, clinic *Clinic) (*Clinic, error) {
	return s.repository.Update(ctx, id, clinic)
}

func (s *service) Delete(ctx context.Context, id string, metadata deletions.Metadata) error {
	return s.repository.Delete(ctx, id, metadata)
}

func (s *service) UpsertAdmin(ctx context.Context, clinicId string, clinicianId string) error {
	return s.repository.UpsertAdmin(ctx, clinicId, clinicianId)
}

func (s *service) RemoveAdmin(ctx context.Context, clinicId string, clinicianId string, allowOrphaning bool) error {
	return s.repository.RemoveAdmin(ctx, clinicId, clinicianId, allowOrphaning)
}

func (s *service) UpdateTier(ctx context.Context, clinicId string, tier string) error {
	return s.repository.UpdateTier(ctx, clinicId, tier)
}

func (s *service) UpdateSuppressedNotifications(ctx context.Context, clinicId string, suppressedNotifications SuppressedNotifications) error {
	return s.repository.UpdateSuppressedNotifications(ctx, clinicId, suppressedNotifications)
}

func (s *service) CreatePatientTag(ctx context.Context, clinicId string, tagName string) (*Clinic, error) {
	return s.repository.CreatePatientTag(ctx, clinicId, tagName)
}

func (s *service) UpdatePatientTag(ctx context.Context, clinicId string, tagId string, tagName string) (*Clinic, error) {
	return s.repository.UpdatePatientTag(ctx, clinicId, tagId, tagName)
}

func (s *service) DeletePatientTag(ctx context.Context, clinicId string, tagId string) (*Clinic, error) {
	return s.repository.DeletePatientTag(ctx, clinicId, tagId)
}

func (s *service) ListMembershipRestrictions(ctx context.Context, clinicId string) ([]MembershipRestrictions, error) {
	return s.repository.ListMembershipRestrictions(ctx, clinicId)
}

func (s *service) UpdateMembershipRestrictions(ctx context.Context, clinicId string, restrictions []MembershipRestrictions) error {
	return s.repository.UpdateMembershipRestrictions(ctx, clinicId, restrictions)
}

func (s *service) GetEHRSettings(ctx context.Context, clinicId string) (*EHRSettings, error) {
	if clinic, err := s.repository.Get(ctx, clinicId); err != nil {
		return nil, err
	} else {
		return clinic.EHRSettings, nil
	}
}

func (s *service) UpdateEHRSettings(ctx context.Context, clinicId string, settings *EHRSettings) error {
	return s.repository.UpdateEHRSettings(ctx, clinicId, settings)
}

func (s *service) GetMRNSettings(ctx context.Context, clinicId string) (*MRNSettings, error) {
	if clinic, err := s.repository.Get(ctx, clinicId); err != nil {
		return nil, err
	} else {
		return clinic.MRNSettings, nil
	}
}

func (s *service) UpdateMRNSettings(ctx context.Context, clinicId string, settings *MRNSettings) error {
	return s.repository.UpdateMRNSettings(ctx, clinicId, settings)
}

func (s *service) GetPatientCountSettings(ctx context.Context, clinicId string) (*PatientCountSettings, error) {
	if clinic, err := s.repository.Get(ctx, clinicId); err != nil {
		return nil, err
	} else {
		return clinic.PatientCountSettings, nil
	}
}

func (s *service) UpdatePatientCountSettings(ctx context.Context, clinicId string, settings *PatientCountSettings) error {
	return s.repository.UpdatePatientCountSettings(ctx, clinicId, settings)
}

func (s *service) GetPatientCount(ctx context.Context, clinicId string) (*PatientCount, error) {
	if clinic, err := s.repository.Get(ctx, clinicId); err != nil {
		return nil, err
	} else {
		return clinic.PatientCount, nil
	}
}

func (s *service) UpdatePatientCount(ctx context.Context, clinicId string, patientCount *PatientCount) error {
	return s.repository.UpdatePatientCount(ctx, clinicId, patientCount)
}

func (s *service) AppendShareCodes(ctx context.Context, clinicId string, shareCodes []string) error {
	return s.repository.AppendShareCodes(ctx, clinicId, shareCodes)
}
