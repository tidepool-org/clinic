package patients

import (
	"context"
	"errors"

	"github.com/tidepool-org/clinic/store"
	"go.uber.org/zap"
)

type service struct {
	repo             Repository
	custodialService CustodialService
	logger           *zap.SugaredLogger
}

var _ Service = &service{}

func NewService(repo Repository, custodialService CustodialService, logger *zap.SugaredLogger) (Service, error) {
	return &service{
		repo:             repo,
		custodialService: custodialService,
		logger:           logger,
	}, nil
}

func (s *service) Get(ctx context.Context, clinicId string, userId string) (*Patient, error) {
	return s.repo.Get(ctx, clinicId, userId)
}

func (s *service) List(ctx context.Context, filter *Filter, pagination store.Pagination, sort *store.Sort) (*ListResult, error) {
	return s.repo.List(ctx, filter, pagination, sort)
}

func (s *service) Create(ctx context.Context, patient Patient) (*Patient, error) {
	// Only create new accounts if the custodial user doesn't exist already (i.e. we are not migrating it)
	if patient.IsCustodial() && patient.UserId == nil {
		s.logger.Infow("creating custodial account", "clinicId", patient.ClinicId.Hex())
		userId, err := s.custodialService.CreateAccount(ctx, patient)
		if err != nil {
			return nil, err
		}
		patient.UserId = &userId
	}

	if patient.UserId == nil {
		return nil, errors.New("user id is missing")
	}

	s.logger.Infow("creating patient in clinic", "userId", patient.UserId, "clinicId", patient.ClinicId.Hex())
	return s.repo.Create(ctx, patient)
}

func (s *service) Update(ctx context.Context, update PatientUpdate) (*Patient, error) {
	existing, err := s.Get(ctx, update.ClinicId, update.UserId)
	if err != nil {
		return nil, err
	}

	if existing.IsCustodial() {
		s.logger.Infow("updating custodial account", "userId", existing.UserId, "clinicId", update.ClinicId)
		update.Patient.ClinicId = existing.ClinicId
		update.Patient.UserId = existing.UserId
		if shouldUpdateInvitedBy(*existing, update) {
			update.Patient.InvitedBy = getUpdatedBy(update)
		}
		if err = s.custodialService.UpdateAccount(ctx, update.Patient); err != nil {
			return nil, err
		}
	}

	s.logger.Infow("updating patient", "userId", existing.UserId, "clinicId", update.ClinicId)
	return s.repo.Update(ctx, update)
}

func (s *service) UpdateEmail(ctx context.Context, userId string, email *string) error {
	s.logger.Infow("updating patient email", "userId", userId, "email", email)
	return s.repo.UpdateEmail(ctx, userId, email)
}

func (s *service) Remove(ctx context.Context, clinicId string, userId string) error {
	s.logger.Infow("deleting patient from clinic", "userId", userId, "clinicId", clinicId)
	return s.repo.Remove(ctx, clinicId, userId)
}

func (s *service) UpdatePermissions(ctx context.Context, clinicId, userId string, permissions *Permissions) (*Patient, error) {
	if permissions != nil && permissions.Custodian != nil {
		// Custodian permission cannot be set after patients claimed their accounts
		permissions.Custodian = nil
	}
	if permissions == nil || permissions.Empty() {
		s.logger.Infow(
			"deleting patient from clinic because the patient revoked all permissions",
			"userId", userId, "clinicId", clinicId,
		)
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
		s.logger.Infow(
			"deleting patient from clinic because the patient revoked all permissions",
			"userId", userId, "clinicId", clinicId,
		)
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

func (s *service) DeleteFromAllClinics(ctx context.Context, userId string) error {
	s.logger.Infow("deleting patients from all clinics", "userId", userId)
	return s.repo.DeleteFromAllClinics(ctx, userId)
}

func (s *service) DeleteNonCustodialPatientsOfClinic(ctx context.Context, clinicId string) error {
	s.logger.Infow("deleting all non-custodial patient of clinic", "clinicId", clinicId)
	return s.repo.DeleteNonCustodialPatientsOfClinic(ctx, clinicId)
}

func (s *service) UpdateSummaryInAllClinics(ctx context.Context, userId string, summary *Summary) error {
	s.logger.Infow("updating summaries for user", "userId", userId)
	return s.repo.UpdateSummaryInAllClinics(ctx, userId, summary)
}

func (s *service) UpdateLastUploadReminderTime(ctx context.Context, update *UploadReminderUpdate) (*Patient, error) {
	s.logger.Infow("updating last upload reminder time for user", "clinicId", update.ClinicId, "userId", update.UserId)
	return s.repo.UpdateLastUploadReminderTime(ctx, update)
}

func (s *service) UpdateLastRequestedDexcomConnectTime(ctx context.Context, update *LastRequestedDexcomConnectUpdate) (*Patient, error) {
	s.logger.Infow("updating last requested dexcom connect time for user", "clinicId", update.ClinicId, "userId", update.UserId)
	return s.repo.UpdateLastRequestedDexcomConnectTime(ctx, update)
}

func (s *service) DeletePatientTagFromClinicPatients(ctx context.Context, clinicId, tagId string) error {
	s.logger.Infow("deleting tag from all patients", "clinicId", clinicId, "tagId", tagId)
	return s.repo.DeletePatientTagFromClinicPatients(ctx, clinicId, tagId)
}

func (s *service) UpdatePatientDataSources(ctx context.Context, userId string, dataSources *DataSources) error {
	s.logger.Infow("updating data sources for clinic patients", "userId", userId)
	return s.repo.UpdatePatientDataSources(ctx, userId, dataSources)
}

func shouldRemovePatientFromClinic(patient *Patient) bool {
	if patient != nil {
		return patient.Permissions == nil || patient.Permissions.Empty()
	}
	return false
}

func shouldUpdateInvitedBy(existing Patient, update PatientUpdate) bool {
	return (existing.Email == nil && update.Patient.Email != nil) ||
		(existing.Email != nil && update.Patient.Email == nil) ||
		(existing.Email != nil && update.Patient.Email != nil && *existing.Email != *update.Patient.Email)
}

// func shouldSetLastRequestedDexcomConnect(existing Patient, update PatientUpdate) bool {
// 	if existing.LastRequestedDexcomConnect == nil {
// 		for _, source := range *update.Patient.DataSources {
// 			if source.ProviderName == "dexcom" && source.State == "pending" {
// 				return true
// 			}
// 		}
// 	}
// 	return update.Patient.ResendConnectDexcomRequest
// }

func getUpdatedBy(update PatientUpdate) *string {
	if update.Patient.Email == nil {
		return nil
	}

	return update.Patient.InvitedBy
}
