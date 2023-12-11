package xealth

import (
	"context"
	"errors"
	"fmt"
	errs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/xealth_client"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const preorderDataCollection = "xealth_preorder"
const ordersCollection = "xealth_order"

type Store interface {
	GetPreorderData(ctx context.Context, dataTrackingId string) (*PreorderFormData, error)
	CreatePreorderData(ctx context.Context, data PreorderFormData) error
	CreateOrder(ctx context.Context, order OrderEvent) (*OrderEvent, error)
	GetOrder(ctx context.Context, documentId string) (*OrderEvent, error)
}

type OrderEvent struct {
	Id                *primitive.ObjectID             `bson:"_id,omitempty"`
	EventNotification xealth_client.EventNotification `bson:"eventNotification"`
	OrderData         xealth_client.ReadOrderResponse `bson:"orderData"`
}

type defaultStore struct {
	preorderData *mongo.Collection
	orders       *mongo.Collection
	logger       *zap.SugaredLogger
}

func NewStore(db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (Store, error) {
	store := &defaultStore{
		preorderData: db.Collection(preorderDataCollection),
		orders:       db.Collection(ordersCollection),
		logger:       logger,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return store.Initialize(ctx)
		},
	})

	return store, nil
}

func (s *defaultStore) Initialize(ctx context.Context) error {
	_, err := s.preorderData.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "dataTrackingId", Value: 1},
			},
			Options: options.Index().
				SetName("dataTrackingId").
				SetUnique(true),
		},
	})

	return err
}

func (s *defaultStore) GetPreorderData(ctx context.Context, dataTrackingId string) (*PreorderFormData, error) {
	selector := bson.M{
		"dataTrackingId": dataTrackingId,
	}
	data := &PreorderFormData{}
	err := s.preorderData.FindOne(ctx, selector).Decode(data)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("%w: preorder data not found", errs.NotFound)
	} else if err != nil {
		return nil, err
	}

	return data, nil
}

func (s *defaultStore) CreatePreorderData(ctx context.Context, data PreorderFormData) error {
	if data.DataTrackingId == "" {
		return fmt.Errorf("data tracking id is required")
	}

	s.logger.Debugw("inserting preorder data", "dataTrackingId", data.DataTrackingId)

	res, err := s.preorderData.InsertOne(ctx, data)
	if err != nil {
		return err
	}

	s.logger.Infow(
		"Successfully inserted preorder data",
		"dataTrackingId", data.DataTrackingId,
		"_id", res.InsertedID,
	)

	return nil
}

func (s *defaultStore) CreateOrder(ctx context.Context, order OrderEvent) (*OrderEvent, error) {
	logger := s.logger.With(
		"deploymentId", order.OrderData.OrderInfo.Deployment,
		"orderId", order.OrderData.OrderInfo.OrderId,
	)

	logger.Debug("inserting xealth order event")

	res, err := s.orders.InsertOne(ctx, order)
	if err != nil {
		return nil, err
	}

	s.logger.Infow("Successfully inserted order event", "_id", res.InsertedID)

	return s.GetOrder(ctx, res.InsertedID.(primitive.ObjectID).Hex())
}

func (s *defaultStore) GetOrder(ctx context.Context, id string) (*OrderEvent, error) {
	objId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errs.NotFound
	}

	orderEvent := &OrderEvent{}
	err = s.orders.FindOne(ctx, bson.M{"_id": objId}).Decode(orderEvent)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, errs.NotFound
	} else if err != nil {
		return nil, err
	}

	return orderEvent, nil
}
