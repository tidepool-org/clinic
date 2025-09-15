package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/deletions"
	errors2 "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
)

type service struct {
	config   *config.Config
	dbClient *mongo.Client
	logger   *zap.SugaredLogger

	clinicsService   clinics.Service
	custodialService CustodialService
	patientsRepo     patients.Repository
}

var _ patients.Service = &service{}

func NewService(config *config.Config, repo patients.Repository, clinics clinics.Service, custodialService CustodialService, logger *zap.SugaredLogger, dbClient *mongo.Client) (patients.Service, error) {
	return &service{
		config:           config,
		dbClient:         dbClient,
		logger:           logger,
		clinicsService:   clinics,
		custodialService: custodialService,
		patientsRepo:     repo,
	}, nil
}

func (s *service) Get(ctx context.Context, clinicId string, userId string) (*patients.Patient, error) {
	return s.patientsRepo.Get(ctx, clinicId, userId)
}

func (s *service) Count(ctx context.Context, filter *patients.Filter) (int, error) {
	return s.patientsRepo.Count(ctx, filter)
}

func (s *service) List(ctx context.Context, filter *patients.Filter, pagination store.Pagination, sorts []*store.Sort) (*patients.ListResult, error) {
	return s.patientsRepo.List(ctx, filter, pagination, sorts)
}

func (s *service) Create(ctx context.Context, patient patients.Patient) (*patients.Patient, error) {
	clinicId := patient.ClinicId.Hex()

	if err := s.enforceMrnSettings(ctx, clinicId, patient.UserId, &patient); err != nil {
		return nil, err
	}
	if err := s.enforcePatientCountSettings(ctx, clinicId, &patient); err != nil {
		return nil, err
	}

	// Only create new accounts if the custodial user doesn't exist already (i.e. we are not migrating it)
	if patient.IsCustodial() && patient.UserId == nil {
		s.logger.Infow("creating custodial account", "clinicId", clinicId)
		userId, err := s.custodialService.CreateAccount(ctx, patient)
		if err != nil {
			return nil, err
		}
		patient.UserId = &userId
	}

	if patient.UserId == nil {
		return nil, errors.New("user id is missing")
	}

	s.logger.Infow("creating patient in clinic", "userId", patient.UserId, "clinicId", clinicId)

	result, err := s.patientsRepo.Create(ctx, patient)

	_ = s.clinicsService.RefreshPatientCount(ctx, clinicId) // Ignore any error, already logged

	return result, err
}

func (s *service) Update(ctx context.Context, update patients.PatientUpdate) (*patients.Patient, error) {
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
	patient, err := s.patientsRepo.Update(ctx, update)
	if err != nil {
		return nil, err
	}

	// Updates to the demo patient user should not affect the patient count
	if update.UserId != s.config.ClinicDemoPatientUserId {
		_ = s.clinicsService.RefreshPatientCount(ctx, update.ClinicId) // Ignore any error, already logged
	}

	return patient, nil
}
func (s *service) AddReview(ctx context.Context, clinicId, userId string, review patients.Review) ([]patients.Review, error) {
	return s.patientsRepo.AddReview(ctx, clinicId, userId, review)
}

func (s *service) DeleteReview(ctx context.Context, clinicId, clinicianId, userId string) ([]patients.Review, error) {
	return s.patientsRepo.DeleteReview(ctx, clinicId, clinicianId, userId)
}

func (s *service) UpdateEmail(ctx context.Context, userId string, email *string) error {
	s.logger.Infow("updating patient email", "userId", userId, "email", email)
	return s.patientsRepo.UpdateEmail(ctx, userId, email)
}

func (s *service) Remove(ctx context.Context, clinicId string, userId string, metadata deletions.Metadata) error {
	s.logger.Infow("deleting patient from clinic", "userId", userId, "clinicId", clinicId)
	_, err := store.WithTransaction(ctx, s.dbClient, func(sessionCtx mongo.SessionContext) (interface{}, error) {
		err := s.patientsRepo.Remove(sessionCtx, clinicId, userId, metadata)
		return nil, err
	})
	if err != nil {
		return err
	}

	_ = s.clinicsService.RefreshPatientCount(ctx, clinicId) // Ignore any error, already logged

	return nil
}

func (s *service) UpdatePermissions(ctx context.Context, clinicId, userId string, permissions *patients.Permissions) (*patients.Patient, error) {
	res, err := store.WithTransaction(ctx, s.dbClient, func(sessionCtx mongo.SessionContext) (interface{}, error) {
		if permissions != nil && permissions.Custodian != nil {
			// Custodian permission cannot be set after patients claimed their accounts
			permissions.Custodian = nil
		}
		if permissions == nil || permissions.Empty() {
			s.logger.Infow(
				"deleting patient from clinic because the patient revoked all permissions",
				"userId", userId, "clinicId", clinicId,
			)
			return nil, s.Remove(ctx, clinicId, userId, deletions.Metadata{DeletedByUserId: &userId})
		}
		return s.patientsRepo.UpdatePermissions(ctx, clinicId, userId, permissions)
	})
	if err != nil || res == nil {
		return nil, err
	}

	return res.(*patients.Patient), nil
}

func (s *service) DeletePermission(ctx context.Context, clinicId, userId, permission string) (*patients.Patient, error) {
	patient, err := s.patientsRepo.DeletePermission(ctx, clinicId, userId, permission)
	if err != nil {
		return nil, err
	}
	if shouldRemovePatientFromClinic(patient) {
		s.logger.Infow(
			"deleting patient from clinic because the patient revoked all permissions",
			"userId", userId, "clinicId", clinicId,
		)
		if err := s.Remove(ctx, clinicId, userId, deletions.Metadata{DeletedByUserId: &userId}); err != nil {
			// the patient was removed by concurrent request which is not a problem,
			// because it had to be removed as a result of the current operation
			if errors.Is(err, patients.ErrNotFound) {
				return nil, nil
			}
			return nil, err
		}
		return nil, nil
	}
	return patient, err
}

func (s *service) DeleteFromAllClinics(ctx context.Context, userId string, metadata deletions.Metadata) ([]string, error) {
	s.logger.Infow("deleting patients from all clinics", "userId", userId)
	res, err := store.WithTransaction(ctx, s.dbClient, func(sessionCtx mongo.SessionContext) (interface{}, error) {
		clinicIds, err := s.patientsRepo.DeleteFromAllClinics(ctx, userId, metadata)
		for _, clinicId := range clinicIds {
			_ = s.clinicsService.RefreshPatientCount(ctx, clinicId) // Ignore any error, already logged
		}
		return clinicIds, err
	})

	if err != nil {
		return nil, err
	}

	return res.([]string), nil
}

func (s *service) DeleteNonCustodialPatientsOfClinic(ctx context.Context, clinicId string, metadata deletions.Metadata) error {
	s.logger.Infow("deleting all non-custodial patient of clinic", "clinicId", clinicId)
	_, err := store.WithTransaction(ctx, s.dbClient, func(sessionCtx mongo.SessionContext) (interface{}, error) {
		err := s.patientsRepo.DeleteNonCustodialPatientsOfClinic(ctx, clinicId, metadata)
		if err != nil {
			return nil, err
		}

		_ = s.clinicsService.RefreshPatientCount(ctx, clinicId) // Ignore any error, already logged

		return nil, nil
	})

	return err
}

func (s *service) UpdateSummaryInAllClinics(ctx context.Context, userId string, summary *patients.Summary) error {
	s.logger.Infow("updating summaries for user", "userId", userId)
	return s.patientsRepo.UpdateSummaryInAllClinics(ctx, userId, summary)
}

func (s *service) DeleteSummaryInAllClinics(ctx context.Context, summaryId string) error {
	s.logger.Infow("deleting summaries matching object id", "objectId", summaryId)
	return s.patientsRepo.DeleteSummaryInAllClinics(ctx, summaryId)
}

func (s *service) UpdateLastUploadReminderTime(ctx context.Context, update *patients.UploadReminderUpdate) (*patients.Patient, error) {
	s.logger.Infow("updating last upload reminder time for user", "clinicId", update.ClinicId, "userId", update.UserId)
	return s.patientsRepo.UpdateLastUploadReminderTime(ctx, update)
}

func (s *service) AddProviderConnectionRequest(ctx context.Context, clinicId, userId string, request patients.ConnectionRequest) error {
	s.logger.Infow("adding provider connection request for user", "clinicId", clinicId, "userId", userId, "provider", request.ProviderName)

	if err := s.patientsRepo.AddProviderConnectionRequest(ctx, clinicId, userId, request); err != nil {
		return err
	}

	// Updates to the demo patient user should not affect the patient count
	if userId != s.config.ClinicDemoPatientUserId {
		_ = s.clinicsService.RefreshPatientCount(ctx, clinicId) // Ignore any error, already logged
	}

	return nil
}

func (s *service) AssignPatientTagToClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error {
	s.logger.Infow("assigning tag to patients", "clinicId", clinicId, "tagId", tagId)
	return s.patientsRepo.AssignPatientTagToClinicPatients(ctx, clinicId, tagId, patientIds)
}

func (s *service) DeletePatientTagFromClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error {
	target := "all"
	if patientIds != nil {
		target = "subset"
	}

	s.logger.Infow("deleting tag from patients", "clinicId", clinicId, "tagId", tagId, "target", target)
	return s.patientsRepo.DeletePatientTagFromClinicPatients(ctx, clinicId, tagId, patientIds)
}

func (s *service) UpdatePatientDataSources(ctx context.Context, userId string, dataSources *patients.DataSources) error {
	s.logger.Infow("updating data sources for clinic patients", "userId", userId)
	if err := s.patientsRepo.UpdatePatientDataSources(ctx, userId, dataSources); err != nil {
		return err
	}

	// Updates to the demo patient user should not affect the patient count
	if userId != s.config.ClinicDemoPatientUserId {

		// Get all clinic ids for this user
		clinicIds, err := s.patientsRepo.GetClinicIds(ctx, userId)
		if err != nil {
			s.logger.Errorw("unable to get clinic ids for user to refresh patient counts", "userId", userId, "error", err)
			return nil
		}

		// Update the patient counts for the clinics
		for _, clinicId := range clinicIds {
			_ = s.clinicsService.RefreshPatientCount(ctx, clinicId) // Ignore any error, already logged
		}
	}

	return nil
}

func (s *service) UpdateEHRSubscription(ctx context.Context, clinicId, userId string, update patients.SubscriptionUpdate) error {
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
	return s.patientsRepo.UpdateEHRSubscription(ctx, clinicId, userId, update)
}

func (s *service) RescheduleLastSubscriptionOrderForAllPatients(ctx context.Context, clinicId, subscription, ordersCollection, targetCollection string) error {
	s.logger.Infow("rescheduling all patient subscriptions", "subscription", subscription, "clinicId", clinicId)
	return s.patientsRepo.RescheduleLastSubscriptionOrderForAllPatients(ctx, clinicId, subscription, ordersCollection, targetCollection)
}

func (s *service) RescheduleLastSubscriptionOrderForPatient(ctx context.Context, clinicIds []string, userId, subscription, ordersCollection, targetCollection string) error {
	s.logger.Infow("rescheduling patient subscriptions", "subscription", subscription, "clinicIds", strings.Join(clinicIds, ", "), "userId", userId)
	return s.patientsRepo.RescheduleLastSubscriptionOrderForPatient(ctx, clinicIds, userId, subscription, ordersCollection, targetCollection)
}

func (s *service) enforceMrnSettings(ctx context.Context, clinicId string, existingUserId *string, patient *patients.Patient) error {
	mrnSettings, err := s.clinicsService.GetMRNSettings(ctx, clinicId)
	if err != nil || mrnSettings == nil {
		return err
	}

	if mrnSettings.Required && (patient.Mrn == nil || *patient.Mrn == "") {
		return fmt.Errorf("%w: mrn is required", errors2.BadRequest)
	}
	if mrnSettings.Unique {
		patient.RequireUniqueMrn = true
		if patient.Mrn != nil {
			filter := &patients.Filter{
				ClinicId: &clinicId,
				Mrn:      patient.Mrn,
			}
			res, err := s.patientsRepo.List(ctx, filter, store.Pagination{Limit: 2, Offset: 0}, nil)
			if err != nil {
				return err
			}

			// The same MRN shouldn't exist already, or it should belong to the same user
			if !(res.MatchingCount == 0 || (res.MatchingCount == 1 && existingUserId != nil && *existingUserId == *res.Patients[0].UserId)) {
				return fmt.Errorf("%w: mrn must be unique", errors2.BadRequest)
			}
		}
	}

	return nil
}

func (s *service) enforcePatientCountSettings(ctx context.Context, clinicId string, patient *patients.Patient) error {

	// Allow non-custodial patients no matter what
	if !patient.IsCustodial() {
		return nil
	}

	// Get any clinic patient count settings, if none found, then no limits, so allow
	patientCountSettings, err := s.clinicsService.GetPatientCountSettings(ctx, clinicId)
	if err != nil || patientCountSettings == nil {
		return err
	}

	// If no patient count hard limit setting, then allow
	if patientCountSettings.HardLimit == nil {
		return nil
	}

	// If outside start date and end date, if specified, then allow
	now := time.Now()
	if patientCountSettings.HardLimit.StartDate != nil && now.Before(*patientCountSettings.HardLimit.StartDate) {
		return nil
	} else if patientCountSettings.HardLimit.EndDate != nil && now.After(*patientCountSettings.HardLimit.EndDate) {
		return nil
	}

	// Get the current clinic patient count
	patientCount, err := s.clinicsService.GetPatientCount(ctx, clinicId)
	if err != nil {
		return err
	} else if patientCount == nil {
		return fmt.Errorf("%w: patient count missing", errors2.InternalServerError)
	}

	// If patient count equals or exceeds patient count hard limit setting, then error
	if patientCount.PatientCount >= patientCountSettings.HardLimit.PatientCount {
		return fmt.Errorf("%w: patient count exceeds limit", errors2.PaymentRequired)
	}

	return nil
}

func shouldRemovePatientFromClinic(patient *patients.Patient) bool {
	if patient != nil {
		return patient.Permissions == nil || patient.Permissions.Empty()
	}
	return false
}

func shouldUpdateInvitedBy(existing patients.Patient, update patients.PatientUpdate) bool {
	return (existing.Email == nil && update.Patient.Email != nil) ||
		(existing.Email != nil && update.Patient.Email == nil) ||
		(existing.Email != nil && update.Patient.Email != nil && *existing.Email != *update.Patient.Email)
}

func getUpdatedBy(update patients.PatientUpdate) *string {
	if update.Patient.Email == nil {
		return nil
	}

	return update.Patient.InvitedBy
}

func (s *service) TideReport(ctx context.Context, clinicId string, params patients.TideReportParams) (*patients.Tide, error) {
	return s.patientsRepo.TideReport(ctx, clinicId, params)
}

func mrnChanged(existing patients.Patient, updated patients.Patient) bool {
	return (existing.Mrn == nil && updated.Mrn != nil) ||
		(existing.Mrn != nil && updated.Mrn == nil) ||
		(existing.Mrn != nil && updated.Mrn != nil && *existing.Mrn != *updated.Mrn)
}

func deactiveAllSubscriptions(subscriptions patients.EHRSubscriptions) patients.EHRSubscriptions {
	for name, sub := range subscriptions {
		sub.Active = false
		subscriptions[name] = sub
	}
	return subscriptions
}
