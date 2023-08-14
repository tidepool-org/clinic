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
)

const (
	verificationTokenHeader = "verification-token"
	collection              = "redox"
)

type Config struct {
	VerificationToken string `envconfig:"TIDEPOOL_REDOX_VERIFICATION_TOKEN" required:"true"`
}

type Redox interface {
	VerifyEndpoint(request VerificationRequest) (*VerificationResponse, error)
	AuthorizeRequest(req *http.Request) error
	ProcessEHRMessage(ctx context.Context, raw []byte) error
	FindMessage(ctx context.Context, documentId, dataModel, eventType string) (*models.MessageEnvelope, error)
	MatchNewOrderToPatient(ctx context.Context, clinic clinics.Clinic, order models.NewOrder, update *patients.SubscriptionUpdate) ([]*patients.Patient, error)
	FindMatchingClinic(ctx context.Context, criteria ClinicMatchingCriteria) (*clinics.Clinic, error)
}

func NewConfig() (Config, error) {
	cfg := Config{}
	err := envconfig.Process("", &cfg)
	return cfg, err
}
func NewHandler(config Config, clinics clinics.Service, patients patients.Service, db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (Redox, error) {
	handler := &Handler{
		collection: db.Collection(collection),
		config:     config,
		logger:     logger,

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
	config     Config
	collection *mongo.Collection
	logger     *zap.SugaredLogger

	clinics  clinics.Service
	patients patients.Service
}

func (h *Handler) Initialize(ctx context.Context) error {
	_, err := h.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
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

	res, err := h.collection.InsertOne(ctx, envelope)
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
	err = h.collection.FindOne(ctx, filter).Decode(&envelope)
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

func (h *Handler) MatchNewOrderToPatient(ctx context.Context, clinic clinics.Clinic, order models.NewOrder, update *patients.SubscriptionUpdate) ([]*patients.Patient, error) {
	criteria, err := GetPatientMatchingCriteriaFromNewOrder(order, clinic)
	if err != nil {
		return nil, err
	}
	if criteria == nil {
		return nil, nil
	}

	filter := patients.Filter{
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

type VerificationRequest struct {
	VerificationToken string `json:"verification-token"`
	Challenge         string `json:"challenge"`
}

type VerificationResponse struct {
	Challenge string `json:"challenge"`
}

type PatientMatchingCriteria struct {
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
	var mrn string
	var dob string

	mrnIdType := clinic.EHRSettings.GetMrnIDType()
	for _, identifier := range order.Patient.Identifiers {
		if identifier.IDType == mrnIdType {
			mrn = identifier.ID
		}
	}
	if mrn == "" {
		return nil, nil
	}

	if order.Patient.Demographics != nil && order.Patient.Demographics.DOB != nil {
		dob = *order.Patient.Demographics.DOB
	}
	if dob == "" {
		return nil, fmt.Errorf("%w: date of birth is missing", errors.BadRequest)
	}

	return &PatientMatchingCriteria{
		Mrn:         mrn,
		DateOfBirth: dob,
	}, nil
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
	if clinic.EHRSettings == nil || order.Order.Procedure == nil || order.Order.Procedure.Code == nil {
		return nil
	}

	update := patients.SubscriptionUpdate{
		MatchedMessage: patients.MatchedMessage{
			DocumentId: documentId,
			DataModel:  order.Meta.DataModel,
			EventType:  order.Meta.EventType,
		},
	}

	switch *order.Order.Procedure.Code {
	case clinic.EHRSettings.ProcedureCodes.EnableSummaryReports:
		update.Name = patients.SummaryAndReportsSubscription
		update.Active = true
		return &update
	case clinic.EHRSettings.ProcedureCodes.DisableSummaryReports:
		update.Name = patients.SummaryAndReportsSubscription
		update.Active = false
		return &update
	}

	return nil
}
