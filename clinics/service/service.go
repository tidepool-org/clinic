package service

import (
	"context"

	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/store"
)

func NewService(repository clinics.Repository, patientsRepository patients.Repository, logger *zap.SugaredLogger) (clinics.Service, error) {
	return &service{
		repository:         repository,
		patientsRepository: patientsRepository,
		logger:             logger,
	}, nil
}

type service struct {
	repository         clinics.Repository
	patientsRepository patients.Repository
	logger             *zap.SugaredLogger
}

func (s *service) Get(ctx context.Context, id string) (*clinics.Clinic, error) {
	return s.repository.Get(ctx, id)
}

func (s *service) List(ctx context.Context, filter *clinics.Filter, pagination store.Pagination) ([]*clinics.Clinic, error) {
	return s.repository.List(ctx, filter, pagination)
}

func (s *service) Create(ctx context.Context, clinic *clinics.Clinic) (*clinics.Clinic, error) {
	return s.repository.Create(ctx, clinic)
}

func (s *service) Update(ctx context.Context, id string, clinic *clinics.Clinic) (*clinics.Clinic, error) {
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

func (s *service) UpdateSuppressedNotifications(ctx context.Context, clinicId string, suppressedNotifications clinics.SuppressedNotifications) error {
	return s.repository.UpdateSuppressedNotifications(ctx, clinicId, suppressedNotifications)
}

func (s *service) CreatePatientTag(ctx context.Context, clinicId string, tagName string) (*clinics.PatientTag, error) {
	return s.repository.CreatePatientTag(ctx, clinicId, tagName)
}

func (s *service) UpdatePatientTag(ctx context.Context, clinicId string, tagId string, tagName string) (*clinics.PatientTag, error) {
	return s.repository.UpdatePatientTag(ctx, clinicId, tagId, tagName)
}

func (s *service) DeletePatientTag(ctx context.Context, clinicId string, tagId string) error {
	return s.repository.DeletePatientTag(ctx, clinicId, tagId)
}

func (s *service) ListMembershipRestrictions(ctx context.Context, clinicId string) ([]clinics.MembershipRestrictions, error) {
	clinic, err := s.repository.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}

	return clinic.MembershipRestrictions, nil
}

func (s *service) UpdateMembershipRestrictions(ctx context.Context, clinicId string, restrictions []clinics.MembershipRestrictions) error {
	return s.repository.UpdateMembershipRestrictions(ctx, clinicId, restrictions)
}

func (s *service) GetEHRSettings(ctx context.Context, clinicId string) (*clinics.EHRSettings, error) {
	if clinic, err := s.repository.Get(ctx, clinicId); err != nil {
		return nil, err
	} else {
		return clinic.EHRSettings, nil
	}
}

func (s *service) UpdateEHRSettings(ctx context.Context, clinicId string, settings *clinics.EHRSettings) error {
	return s.repository.UpdateEHRSettings(ctx, clinicId, settings)
}

func (s *service) GetMRNSettings(ctx context.Context, clinicId string) (*clinics.MRNSettings, error) {
	if clinic, err := s.repository.Get(ctx, clinicId); err != nil {
		return nil, err
	} else {
		return clinic.MRNSettings, nil
	}
}

func (s *service) UpdateMRNSettings(ctx context.Context, clinicId string, settings *clinics.MRNSettings) error {
	return s.repository.UpdateMRNSettings(ctx, clinicId, settings)
}

func (s *service) GetPatientCountSettings(ctx context.Context, clinicId string) (*clinics.PatientCountSettings, error) {
	if clinic, err := s.repository.Get(ctx, clinicId); err != nil {
		return nil, err
	} else {
		return clinic.PatientCountSettings, nil
	}
}

func (s *service) UpdatePatientCountSettings(ctx context.Context, clinicId string, settings *clinics.PatientCountSettings) error {
	return s.repository.UpdatePatientCountSettings(ctx, clinicId, settings)
}

func (s *service) GetPatientCount(ctx context.Context, clinicId string) (*clinics.PatientCount, error) {
	if clinic, err := s.repository.Get(ctx, clinicId); err != nil {
		return nil, err
	} else if clinic.PatientCount != nil {
		return clinic.PatientCount, nil
	}

	if err := s.RefreshPatientCount(ctx, clinicId); err != nil {
		return nil, err
	}

	if clinic, err := s.repository.Get(ctx, clinicId); err != nil {
		return nil, err
	} else {
		return clinic.PatientCount, nil
	}
}

func (s *service) RefreshPatientCount(ctx context.Context, clinicId string) error {
	counts, err := s.patientsRepository.Counts(ctx, clinicId)
	if err != nil {
		s.logger.Errorf("Failed to refresh patient count for clinic %s: %v", clinicId, err)
		return err
	}

	patientCount := &clinics.PatientCount{
		Total: counts.Total,
		Demo:  counts.Demo,
		Plan:  counts.Plan,
	}
	if counts.Providers != nil {
		patientCount.Providers = make(map[string]clinics.PatientProviderCount, len(counts.Providers))
		for provider, providerPatientCount := range counts.Providers {
			patientCount.Providers[provider] = clinics.PatientProviderCount{
				States: providerPatientCount.States,
				Total:  providerPatientCount.Total,
			}
		}
	}

	err = s.repository.UpdatePatientCount(ctx, clinicId, patientCount)
	if err != nil {
		s.logger.Errorf("Failed to update patient count for clinic %s: %v", clinicId, err)
		return err
	}

	return nil
}

func (s *service) AppendShareCodes(ctx context.Context, clinicId string, shareCodes []string) error {
	return s.repository.AppendShareCodes(ctx, clinicId, shareCodes)
}

func (s *service) CreateSite(ctx context.Context, clinicId string, site *sites.Site) (*sites.Site, error) {
	return s.repository.CreateSite(ctx, clinicId, site)
}

func (s *service) CreateSiteIgnoringLimit(ctx context.Context, clinicId string, site *sites.Site) (*sites.Site, error) {
	return s.repository.CreateSiteIgnoringLimit(ctx, clinicId, site)
}

func (s *service) DeleteSite(ctx context.Context, clinicId, siteId string) error {
	return s.repository.DeleteSite(ctx, clinicId, siteId)
}

func (s *service) UpdateSite(ctx context.Context, clinicId, siteId string, site *sites.Site) (*sites.Site, error) {
	return s.repository.UpdateSite(ctx, clinicId, siteId, site)
}
