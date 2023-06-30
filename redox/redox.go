package redox

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/tidepool-org/clinic/errors"
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
}

func NewConfig() (Config, error) {
	cfg := Config{}
	err := envconfig.Process("", &cfg)
	return cfg, err
}
func NewHandler(config Config, db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (Redox, error) {
	handler := &Handler{
		collection: db.Collection(collection),
		config:     config,
		logger:     logger,
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
}

func (r *Handler) Initialize(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
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
			},
			Options: options.Index().
				SetUnique(true).
				SetName("MetadataSourceId"),
		},
		{
			Keys: bson.D{
				{Key: "Meta.FacilityCode", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("MetadataFacilityCode"),
		},
	})

	return err
}

func (r *Handler) VerifyEndpoint(request VerificationRequest) (*VerificationResponse, error) {
	if request.VerificationToken != r.config.VerificationToken {
		return nil, fmt.Errorf("%w: invalid verification token", errors.Unauthorized)
	}

	return &VerificationResponse{Challenge: request.Challenge}, nil
}

func (r *Handler) AuthorizeRequest(req *http.Request) error {
	verificationToken := req.Header.Get(verificationTokenHeader)

	if verificationToken == "" {
		return fmt.Errorf("%w: verification token is required", errors.Unauthorized)
	}
	if verificationToken != r.config.VerificationToken {
		return fmt.Errorf("%w: invalid verification token", errors.Unauthorized)
	}

	return nil
}

func (r *Handler) ProcessEHRMessage(ctx context.Context, raw []byte) error {
	// Deserialize request metadata, so it's easier to process it later
	message := struct {
		Meta Meta
	}{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}

	if !message.Meta.IsValid() {
		r.logger.Errorw(
			"unable to process message with invalid metadata",
			"metadata", message.Meta,
		)
		return fmt.Errorf("%w: cannot process message with invalid metadata", errors.BadRequest)
	}

	bsonRaw := bson.Raw{}
	if err := bson.UnmarshalExtJSON(raw, true, &bsonRaw); err != nil {
		r.logger.Errorw("unable to unmarshal raw json")
		return fmt.Errorf("%w: unable to unmarshall message", errors.BadRequest)
	}

	envelope := MessageEnvelope{
		Meta:    message.Meta,
		Message: bsonRaw,
	}

	r.logger.Debugw("saving EHR message to database", "metadata", message.Meta)

	res, err := r.collection.InsertOne(ctx, envelope)
	if err != nil {
		return err
	}

	r.logger.Infow(
		"Successfully inserted EHR message",
		"metadata", message.Meta,
		"_id", res.InsertedID,
	)

	return nil
}
