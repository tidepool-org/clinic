package patients

import (
	"context"
	"errors"
	"fmt"
	"github.com/tidepool-org/clinic/clinics"
	errors2 "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.uber.org/zap"
)

type service struct {
	clinics          clinics.Service
	repo             Repository
	custodialService CustodialService
	logger           *zap.SugaredLogger
}

var _ Service = &service{}

func NewService(repo Repository, clinics clinics.Service, custodialService CustodialService, logger *zap.SugaredLogger) (Service, error) {
	return &service{
		clinics:          clinics,
		repo:             repo,
		custodialService: custodialService,
		logger:           logger,
	}, nil
}

func (s *service) Get(ctx context.Context, clinicId string, userId string) (*Patient, error) {
	return s.repo.Get(ctx, clinicId, userId)
}

func (s *service) List(ctx context.Context, filter *Filter, pagination store.Pagination, sorts []*store.Sort) (*ListResult, error) {
	return s.repo.List(ctx, filter, pagination, sorts)
}

func (s *service) Create(ctx context.Context, patient Patient) (*Patient, error) {
	if err := s.enforceMrnSettings(ctx, patient.ClinicId.Hex(), patient.UserId, &patient); err != nil {
		return nil, err
	}

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

	if err := s.enforceMrnSettings(ctx, update.ClinicId, &update.UserId, &update.Patient); err != nil {
		return nil, err
	}

	if mrnChanged(*existing, update.Patient) {
		update.Patient.EHRSubscriptions = deactiveAllSubscriptions(existing.EHRSubscriptions)
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
			if errors.Is(err, ErrNotFound) {
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

func (s *service) AssignPatientTagToClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error {
	s.logger.Infow("assigning tag to patients", "clinicId", clinicId, "tagId", tagId)
	return s.repo.AssignPatientTagToClinicPatients(ctx, clinicId, tagId, patientIds)
}

func (s *service) DeletePatientTagFromClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error {
	target := "all"
	if patientIds != nil {
		target = "subset"
	}

	s.logger.Infow("deleting tag from patients", "clinicId", clinicId, "tagId", tagId, "target", target)
	return s.repo.DeletePatientTagFromClinicPatients(ctx, clinicId, tagId, patientIds)
}

func (s *service) UpdatePatientDataSources(ctx context.Context, userId string, dataSources *DataSources) error {
	s.logger.Infow("updating data sources for clinic patients", "userId", userId)
	return s.repo.UpdatePatientDataSources(ctx, userId, dataSources)
}

func (s *service) UpdateEHRSubscription(ctx context.Context, clinicId, userId string, update SubscriptionUpdate) error {
	patient, err := s.Get(ctx, clinicId, userId)
	if err != nil {
		return err
	}

	// Check if this message has already been matched
	if patient.EHRSubscriptions != nil {
		if subscr, ok := patient.EHRSubscriptions[update.Name]; ok {
			for _, msg := range subscr.MatchedMessages {
				if update.MatchedMessage.DocumentId == msg.DocumentId {
					s.logger.Infow("the message has already been matched, skipping update", "clinicId", clinicId, "userId", userId, "update", update)
					return nil
				}
			}
		}
	}

	s.logger.Infow("updating patient subscription", "clinicId", clinicId, "userId", userId, "update", update)
	return s.repo.UpdateEHRSubscription(ctx, clinicId, userId, update)
}

func (s *service) RescheduleLastSubscriptionOrderForAllPatients(ctx context.Context, clinicId, subscription, ordersCollection, targetCollection string) error {
	s.logger.Infow("rescheduling all patient subscriptions", "subscription", subscription, "clinicId", clinicId)
	return s.repo.RescheduleLastSubscriptionOrderForAllPatients(ctx, clinicId, subscription, ordersCollection, targetCollection)
}

func (s *service) RescheduleLastSubscriptionOrderForPatient(ctx context.Context, userId, subscription, ordersCollection, targetCollection string) error {
	s.logger.Infow("rescheduling patient subscriptions", "subscription", subscription, "userId", userId)
	return s.repo.RescheduleLastSubscriptionOrderForPatient(ctx, userId, subscription, ordersCollection, targetCollection)
}

func (s *service) enforceMrnSettings(ctx context.Context, clinicId string, existingUserId *string, patient *Patient) error {
	mrnSettings, err := s.clinics.GetMRNSettings(ctx, clinicId)
	if err != nil || mrnSettings == nil {
		return err
	}

	if mrnSettings.Required && (patient.Mrn == nil || *patient.Mrn == "") {
		return fmt.Errorf("%w: mrn is required", errors2.BadRequest)
	}
	if mrnSettings.Unique {
		patient.RequireUniqueMrn = true
		if patient.Mrn != nil {
			filter := &Filter{
				ClinicId: &clinicId,
				Mrn:      patient.Mrn,
			}
			res, err := s.repo.List(ctx, filter, store.Pagination{Limit: 2, Offset: 0}, nil)
			if err != nil {
				return err
			}

			// The same MRN shouldn't exist already, or it should belong to the same user
			if !(res.TotalCount == 0 || (res.TotalCount == 1 && existingUserId != nil && *existingUserId == *res.Patients[0].UserId)) {
				return fmt.Errorf("%w: mrn must be unique", errors2.BadRequest)
			}
		}
	}

	return nil
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

func getUpdatedBy(update PatientUpdate) *string {
	if update.Patient.Email == nil {
		return nil
	}

	return update.Patient.InvitedBy
}

func (s *service) TideReport(ctx context.Context, clinicId string, params TideReportParams) (*Tide, error) {
	return s.repo.TideReport(ctx, clinicId, params)
}

func mrnChanged(existing Patient, updated Patient) bool {
	return (existing.Mrn == nil && updated.Mrn != nil) ||
		(existing.Mrn != nil && updated.Mrn == nil) ||
		(existing.Mrn != nil && updated.Mrn != nil && *existing.Mrn != *updated.Mrn)
}

func deactiveAllSubscriptions(subscriptions EHRSubscriptions) EHRSubscriptions {
	for name, sub := range subscriptions {
		sub.Active = false
		subscriptions[name] = sub
	}
	return subscriptions
}
