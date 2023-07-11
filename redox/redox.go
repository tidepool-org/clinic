package redox

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/redox/models"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
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
	FindMatchingClinic(ctx context.Context, criteria ClinicMatchingCriteria) (*clinics.Clinic, error)
	MatchPatient(ctx context.Context, criteria PatientMatchingCriteria) ([]*patients.Patient, error)
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
				{Key: "Meta.Logs.Id", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("MetadataLogsId"),
		},
		{
			Keys: bson.D{
				{Key: "Meta.Source.Id", Value: 1},
				{Key: "Meta.FacilityCode", Value: 1},
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

func (h *Handler) MatchPatient(ctx context.Context, criteria PatientMatchingCriteria) ([]*patients.Patient, error) {
	if criteria.Mrn == "" || criteria.DateOfBirth == "" {
		return nil, fmt.Errorf("%w: mrn and birth date are required", errors.BadRequest)
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

	if err == nil && result.TotalCount == 1 && result.Patients[0] != nil {
		// Add the matched EHR identity, so we can push data without an Order later
		match := result.Patients[0]
		match.EHRIdentity = &patients.EHRIdentity{
			FirstName:   criteria.FirstName,
			MiddleName:  criteria.MiddleName,
			LastName:    criteria.LastName,
			DateOfBirth: criteria.DateOfBirth,
			Mrn:         criteria.Mrn,
		}
		updated, err := h.patients.Update(ctx, patients.PatientUpdate{
			ClinicId: match.ClinicId.Hex(),
			UserId:   *match.UserId,
			Patient:  *match,
		})
		if err != nil {
			return nil, err
		}
		result.Patients[0] = updated
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
	FirstName   string
	MiddleName  string
	LastName    string
	Mrn         string
	DateOfBirth string
}

type ClinicMatchingCriteria struct {
	SourceId     string
	FacilityName *string
}
