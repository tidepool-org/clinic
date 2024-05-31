package redox

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	models "github.com/tidepool-org/clinic/redox_models"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
)

const (
	verificationTokenHeader                          = "verification-token"
	messagesCollectionName                           = "redox"
	summaryAndReportsRescheduledOrdersCollectionName = "scheduledSummaryAndReportsOrders"
	rescheduledMessagesExpiration                    = 30 * 24 * time.Hour
)

type Config struct {
	VerificationToken string `envconfig:"TIDEPOOL_REDOX_VERIFICATION_TOKEN"`
}

type Redox interface {
	VerifyEndpoint(request VerificationRequest) (*VerificationResponse, error)
	AuthorizeRequest(req *http.Request) error
	ProcessEHRMessage(ctx context.Context, raw []byte) error
	FindMessage(ctx context.Context, documentId, dataModel, eventType string) (*models.MessageEnvelope, error)
	MatchNewOrderToPatient(ctx context.Context, clinic clinics.Clinic, order models.NewOrder, update *patients.SubscriptionUpdate) ([]*patients.Patient, error)
	FindMatchingClinic(ctx context.Context, criteria ClinicMatchingCriteria) (*clinics.Clinic, error)
	RescheduleSubscriptionOrders(ctx context.Context, clinicId string) error
	RescheduleSubscriptionOrdersForPatient(ctx context.Context, patientId string) error
}

func NewConfig() (Config, error) {
	cfg := Config{}
	err := envconfig.Process("", &cfg)
	return cfg, err
}
func NewHandler(config Config, clinics clinics.Service, patients patients.Service, db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (Redox, error) {
	handler := &Handler{
		messagesCollection:                     db.Collection(messagesCollectionName),
		rescheduledSummaryAndReportsCollection: db.Collection(summaryAndReportsRescheduledOrdersCollectionName),
		config:                                 config,
		logger:                                 logger,

		clinics:  clinics,
		patients: patients,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return handler.Initialize(ctx)
		},
	})

	return handler, nil
}

type Handler struct {
	config                                 Config
	messagesCollection                     *mongo.Collection
	rescheduledSummaryAndReportsCollection *mongo.Collection
	logger                                 *zap.SugaredLogger

	clinics  clinics.Service
	patients patients.Service
}

func (h *Handler) Initialize(ctx context.Context) error {
	_, err := h.messagesCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "meta.Logs.Id", Value: 1},
			},
			Options: options.Index().
				SetName("MetadataLogsId"),
		},
		{
			Keys: bson.D{
				{Key: "meta.Source.Id", Value: 1},
				{Key: "meta.FacilityCode", Value: 1},
			},
			Options: options.Index().
				SetName("MetadataSource"),
		},
	})
	if err != nil {
		return err
	}

	_, err = h.rescheduledSummaryAndReportsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "createdTime", Value: 1},
			},
			Options: options.Index().
				SetExpireAfterSeconds(int32(rescheduledMessagesExpiration.Seconds())).
				SetName("CleanupExpiredRescheduledOrders"),
		},
	})

	return err
}

func (h *Handler) VerifyEndpoint(request VerificationRequest) (*VerificationResponse, error) {
	if request.VerificationToken != h.config.VerificationToken {
		return nil, fmt.Errorf("%w: invalid verification token", errors.Unauthorized)
	}

	return &VerificationResponse{Challenge: request.Challenge}, nil
}

func (h *Handler) AuthorizeRequest(req *http.Request) error {
	verificationToken := req.Header.Get(verificationTokenHeader)

	if verificationToken == "" {
		return fmt.Errorf("%w: verification token is required", errors.Unauthorized)
	}
	if verificationToken != h.config.VerificationToken {
		return fmt.Errorf("%w: invalid verification token", errors.Unauthorized)
	}

	return nil
}

func (h *Handler) ProcessEHRMessage(ctx context.Context, raw []byte) error {
	// Deserialize request metadata, so it's easier to process it later
	message := struct {
		Meta models.Meta
	}{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}

	if !message.Meta.IsValid() {
		h.logger.Errorw(
			"unable to process message with invalid metadata",
			"metadata", message.Meta,
		)
		return fmt.Errorf("%w: cannot process message with invalid metadata", errors.BadRequest)
	}

	bsonRaw := bson.Raw{}
	if err := bson.UnmarshalExtJSON(raw, true, &bsonRaw); err != nil {
		h.logger.Errorw("unable to unmarshal raw json")
		return fmt.Errorf("%w: unable to unmarshall message", errors.BadRequest)
	}

	envelope := models.MessageEnvelope{
		Meta:    message.Meta,
		Message: bsonRaw,
	}

	h.logger.Debugw("saving EHR message to database", "metadata", message.Meta)

	res, err := h.messagesCollection.InsertOne(ctx, envelope)
	if err != nil {
		return err
	}

	h.logger.Infow(
		"Successfully inserted EHR message",
		"metadata", message.Meta,
		"_id", res.InsertedID,
	)

	return nil
}

func (h *Handler) FindMessage(ctx context.Context, documentId, dataModel, eventType string) (*models.MessageEnvelope, error) {
	id, err := primitive.ObjectIDFromHex(documentId)
	if err != nil {
		return nil, err
	}

	envelope := models.MessageEnvelope{}
	filter := bson.M{"_id": id, "meta.DataModel": dataModel, "meta.EventType": eventType}
	err = h.messagesCollection.FindOne(ctx, filter).Decode(&envelope)
	if err != nil {
		return nil, err
	}

	return &envelope, nil
}

func (h *Handler) FindMatchingClinicFromNewOrder(ctx context.Context, order *models.NewOrder) (*clinics.Clinic, error) {
	criteria, err := GetClinicMatchingCriteriaFromNewOrder(order)
	if err != nil {
		return nil, err
	}

	return h.FindMatchingClinic(ctx, criteria)
}

func (h *Handler) FindMatchingClinic(ctx context.Context, criteria ClinicMatchingCriteria) (*clinics.Clinic, error) {
	if criteria.SourceId == "" {
		return nil, fmt.Errorf("%w: source id is required", errors.BadRequest)
	}

	enabled := true
	filter := clinics.Filter{
		EHRProvider:     &clinics.EHRProviderRedox,
		EHRSourceId:     &criteria.SourceId,
		EHRFacilityName: criteria.FacilityName,
		EHREnabled:      &enabled,
	}
	page := store.Pagination{
		Offset: 0,
		Limit:  2,
	}

	result, err := h.clinics.List(ctx, &filter, page)
	if err != nil {
		return nil, err
	}

	if len(result) > 1 {
		return nil, fmt.Errorf("%w: multiple matching clinics found", errors.Duplicate)
	} else if len(result) == 0 || result[0] == nil || result[0].Id == nil {
		return nil, fmt.Errorf("%w: couldn't find a matching clinic", errors.NotFound)
	}

	return result[0], nil
}

func (h *Handler) RescheduleSubscriptionOrders(ctx context.Context, clinicId string) error {
	enabled := true
	filter := clinics.Filter{
		Ids:         []string{clinicId},
		EHRProvider: &clinics.EHRProviderRedox,
		EHREnabled:  &enabled,
	}
	page := store.Pagination{
		Offset: 0,
		Limit:  2,
	}

	result, err := h.clinics.List(ctx, &filter, page)
	if err != nil {
		return err
	}

	if len(result) > 1 {
		return fmt.Errorf("%w: multiple matching clinics found", errors.Duplicate)
	} else if len(result) == 0 || result[0] == nil || result[0].Id == nil {
		return fmt.Errorf("%w: couldn't find a matching clinic", errors.NotFound)
	}

	return h.patients.RescheduleLastSubscriptionOrderForAllPatients(
		ctx,
		clinicId,
		patients.SubscriptionRedoxSummaryAndReports,
		messagesCollectionName,
		summaryAndReportsRescheduledOrdersCollectionName,
	)
}

func (h *Handler) RescheduleSubscriptionOrdersForPatient(ctx context.Context, patientId string) error {
	enabled := true
	limit := 10000
	filter := clinics.Filter{
		EHRProvider: &clinics.EHRProviderRedox,
		EHREnabled:  &enabled,
	}
	page := store.Pagination{
		Offset: 0,
		Limit:  limit,
	}

	clinicIds := make([]string, 0, 100)
	for {
		result, err := h.clinics.List(ctx, &filter, page)
		if err != nil {
			return err
		}
		for _, clinic := range result {
			if clinic != nil && clinic.Id != nil {
				clinicIds = append(clinicIds, clinic.Id.Hex())
			}
		}
		if len(result) < limit {
			break
		}
	}

	return h.patients.RescheduleLastSubscriptionOrderForPatient(
		ctx,
		clinicIds,
		patientId,
		patients.SubscriptionRedoxSummaryAndReports,
		messagesCollectionName,
		summaryAndReportsRescheduledOrdersCollectionName,
	)
}

func (h *Handler) MatchNewOrderToPatient(ctx context.Context, clinic clinics.Clinic, order models.NewOrder, update *patients.SubscriptionUpdate) ([]*patients.Patient, error) {
	if clinic.EHRSettings == nil {
		return nil, fmt.Errorf("%w: clinic has no EHR settings", errors.BadRequest)
	}

	code := GetProcedureCodeFromOrder(order)
	procedureCodes := clinic.EHRSettings.ProcedureCodes
	if (procedureCodes.EnableSummaryReports != nil && code == *procedureCodes.EnableSummaryReports) ||
		(procedureCodes.DisableSummaryReports != nil && code == *procedureCodes.DisableSummaryReports) {
		return h.MatchPatientsForSubscriptionOrder(ctx, clinic, order, update)
	} else if procedureCodes.CreateAccount != nil && code == *procedureCodes.CreateAccount {
		return h.FindMatchingPatientsForAccountCreationOrder(ctx, clinic, order)
	}

	return nil, nil
}

func (h *Handler) MatchPatientsForSubscriptionOrder(ctx context.Context, clinic clinics.Clinic, order models.NewOrder, update *patients.SubscriptionUpdate) ([]*patients.Patient, error) {
	criteria, err := GetPatientMatchingCriteriaFromNewOrder(order, clinic)
	if err != nil {
		return nil, err
	}
	if criteria == nil {
		return nil, nil
	}

	clinicId := clinic.Id.Hex()
	filter := patients.Filter{
		ClinicId:  &clinicId,
		Mrn:       &criteria.Mrn,
		BirthDate: &criteria.DateOfBirth,
	}

	page := store.Pagination{
		Offset: 0,
		Limit:  100,
	}

	result, err := h.patients.List(ctx, &filter, page, nil)

	if err == nil && result.TotalCount == 1 && result.Patients[0] != nil && update != nil {
		// Update the subscription for matched patient only if single match was found
		match := result.Patients[0]
		if err := h.patients.UpdateEHRSubscription(ctx, match.ClinicId.Hex(), *match.UserId, *update); err != nil {
			return nil, err
		}
	}

	return result.Patients, err
}

func (h *Handler) FindMatchingPatientsForAccountCreationOrder(ctx context.Context, clinic clinics.Clinic, order models.NewOrder) ([]*patients.Patient, error) {
	criteria, err := GetPatientMatchingCriteriaFromNewOrder(order, clinic)
	if err != nil {
		return nil, err
	}
	if criteria == nil {
		return nil, nil
	}

	var matchingPatients []*patients.Patient

	clinicId := clinic.Id.Hex()
	page := store.Pagination{
		Offset: 0,
		Limit:  100,
	}

	filter := patients.Filter{
		ClinicId: &clinicId,
		Mrn:      &criteria.Mrn,
	}
	result, err := h.patients.List(ctx, &filter, page, nil)
	if err != nil {
		return nil, err
	}

	unique := map[string]struct{}{}
	if result.TotalCount > 0 {
		for _, patient := range result.Patients {
			if patient == nil || patient.UserId == nil {
				continue
			}
			if _, ok := unique[*patient.UserId]; ok {
				continue
			}
			unique[*patient.UserId] = struct{}{}
			matchingPatients = append(matchingPatients, patient)
		}
	}

	filter = patients.Filter{
		ClinicId:  &clinicId,
		BirthDate: &criteria.DateOfBirth,
		FullName:  &criteria.FullName,
	}
	result, err = h.patients.List(ctx, &filter, page, nil)
	if err != nil {
		return nil, err
	}
	if result.TotalCount > 0 {
		for _, patient := range result.Patients {
			if patient == nil || patient.UserId == nil {
				continue
			}
			if _, ok := unique[*patient.UserId]; ok {
				continue
			}
			unique[*patient.UserId] = struct{}{}
			matchingPatients = append(matchingPatients, patient)
		}
	}

	return matchingPatients, nil
}

type VerificationRequest struct {
	VerificationToken string `json:"verification-token"`
	Challenge         string `json:"challenge"`
}

type VerificationResponse struct {
	Challenge string `json:"challenge"`
}

type PatientMatchingCriteria struct {
	FirstName   string
	LastName    string
	FullName    string
	Mrn         string
	DateOfBirth string
}

type ClinicMatchingCriteria struct {
	SourceId     string
	FacilityName *string
}

func GetClinicMatchingCriteriaFromNewOrder(order *models.NewOrder) (ClinicMatchingCriteria, error) {
	criteria := ClinicMatchingCriteria{}
	if order.Meta.Source == nil || order.Meta.Source.ID == nil || *order.Meta.Source.ID == "" {
		return criteria, fmt.Errorf("%w: source id is required", errors.BadRequest)
	}
	criteria.SourceId = *order.Meta.Source.ID

	if order.Order.OrderingFacility != nil {
		criteria.FacilityName = order.Order.OrderingFacility.Name
	}

	return criteria, nil
}

func GetPatientMatchingCriteriaFromNewOrder(order models.NewOrder, clinic clinics.Clinic) (*PatientMatchingCriteria, error) {
	if clinic.EHRSettings == nil {
		return nil, fmt.Errorf("%w: clinic has no EHR settings", errors.BadRequest)
	}
	criteria := &PatientMatchingCriteria{}

	mrnIdType := clinic.EHRSettings.GetMrnIDType()
	for _, identifier := range order.Patient.Identifiers {
		if identifier.IDType == mrnIdType {
			criteria.Mrn = identifier.ID
		}
	}

	if order.Patient.Demographics != nil {
		names := make([]string, 0, 2)
		if order.Patient.Demographics.DOB != nil {
			criteria.DateOfBirth = *order.Patient.Demographics.DOB
		}
		if order.Patient.Demographics.FirstName != nil {
			criteria.FirstName = *order.Patient.Demographics.FirstName
			names = append(names, criteria.FirstName)
		}
		if order.Patient.Demographics.LastName != nil {
			criteria.LastName = *order.Patient.Demographics.LastName
			names = append(names, criteria.LastName)
		}
		if len(names) > 0 {
			criteria.FullName = strings.Join(names, " ")
		}
	}

	if criteria.Mrn == "" {
		return nil, nil
	}
	if criteria.DateOfBirth == "" {
		return nil, fmt.Errorf("%w: date of birth is missing", errors.BadRequest)
	}
	if criteria.FullName == "" {
		return nil, fmt.Errorf("%w: full name is missing", errors.BadRequest)
	}

	return criteria, nil
}

type Model interface {
	models.NewOrder
}

func UnmarshallMessage[S *T, T Model](envelope models.MessageEnvelope) (S, error) {
	model := new(T)
	if err := bson.Unmarshal(envelope.Message, model); err != nil {
		return nil, err
	}

	return model, nil
}

func GetUpdateFromNewOrder(clinic clinics.Clinic, documentId primitive.ObjectID, order models.NewOrder) *patients.SubscriptionUpdate {
	code := GetProcedureCodeFromOrder(order)
	update := patients.SubscriptionUpdate{
		MatchedMessage: patients.MatchedMessage{
			DocumentId: documentId,
			DataModel:  order.Meta.DataModel,
			EventType:  order.Meta.EventType,
		},
		Provider: clinics.EHRProviderRedox,
	}

	if clinic.EHRSettings.ProcedureCodes.EnableSummaryReports != nil && *clinic.EHRSettings.ProcedureCodes.EnableSummaryReports == code {
		update.Name = patients.SubscriptionRedoxSummaryAndReports
		update.Active = true
		return &update
	} else if clinic.EHRSettings.ProcedureCodes.DisableSummaryReports != nil && *clinic.EHRSettings.ProcedureCodes.DisableSummaryReports == code {
		update.Name = patients.SubscriptionRedoxSummaryAndReports
		update.Active = false
		return &update
	}

	return nil
}

func GetProcedureCodeFromOrder(order models.NewOrder) (code string) {
	if order.Order.Procedure == nil || order.Order.Procedure.Code == nil {
		return
	}

	code = *order.Order.Procedure.Code
	return
}
