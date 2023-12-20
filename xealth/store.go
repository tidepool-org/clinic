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
	"time"
)

const preorderDataCollection = "xealth_preorder"
const ordersCollection = "xealth_order"
const reportViewCollection = "xealth_report_view"

type Store interface {
	GetPreorderData(ctx context.Context, dataTrackingId string) (*PreorderFormData, error)
	CreatePreorderData(ctx context.Context, data PreorderFormData) error
	CreateOrder(ctx context.Context, order OrderEvent) (*OrderEvent, error)
	GetOrder(ctx context.Context, documentId string) (*OrderEvent, error)
	GetReportView(ctx context.Context, documentId string) (*ReportView, error)
	GetMostRecentReportView(ctx context.Context, filter ReportViewFilter) (*ReportView, error)
	CreateReportView(ctx context.Context, view ReportView) (*ReportView, error)
}

type OrderEvent struct {
	Id                *primitive.ObjectID             `bson:"_id,omitempty"`
	EventNotification xealth_client.EventNotification `bson:"eventNotification"`
	OrderData         xealth_client.ReadOrderResponse `bson:"orderData"`
}

type ReportView struct {
	Id            *primitive.ObjectID `bson:"_id,omitempty"`
	UserId        string              `bson:"userId"`
	DeploymentId  string              `bson:"deploymentId"`
	SystemLogin   *string             `bson:"systemLogin,omitempty"`
	PatientUserId string              `bson:"patientUserId"`
	ProgramId     string              `bson:"programId"`
	ClinicId      primitive.ObjectID  `bson:"clinicId"`
	CreatedTime   time.Time           `bson:"createdTime"`
}

type ReportViewFilter struct {
	ClinicId      primitive.ObjectID `bson:"clinicId"`
	DeploymentId  string             `bson:"deploymentId"`
	PatientUserId string             `bson:"patientUserId"`
	ProgramId     string             `bson:"programId"`
	UserId        string             `bson:"userId"`
}

type defaultStore struct {
	orders       *mongo.Collection
	preorderData *mongo.Collection
	reportViews  *mongo.Collection
	logger       *zap.SugaredLogger
}

func NewStore(db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (Store, error) {
	store := &defaultStore{
		orders:       db.Collection(ordersCollection),
		preorderData: db.Collection(preorderDataCollection),
		reportViews:  db.Collection(reportViewCollection),
		logger:       logger,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return store.Initialize(ctx)
		},
	})

	return store, nil
}

func (d *defaultStore) Initialize(ctx context.Context) error {
	_, err := d.preorderData.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "dataTrackingId", Value: 1},
			},
			Options: options.Index().
				SetName("dataTrackingId").
				SetUnique(true),
		},
	})
	if err != nil {
		return err
	}

	_, err = d.reportViews.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "userId", Value: 1},
				{Key: "deploymentId", Value: 1},
				{Key: "programId", Value: 1},
				{Key: "clinicId", Value: 1},
				{Key: "patientId", Value: 1},
				{Key: "createdTime", Value: -1},
			},
			Options: options.Index().
				SetName("LastReportView"),
		},
	})

	return err
}

func (d *defaultStore) GetPreorderData(ctx context.Context, dataTrackingId string) (*PreorderFormData, error) {
	selector := bson.M{
		"dataTrackingId": dataTrackingId,
	}
	data := &PreorderFormData{}
	err := d.preorderData.FindOne(ctx, selector).Decode(data)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("%w: preorder data not found", errs.NotFound)
	} else if err != nil {
		return nil, err
	}

	return data, nil
}

func (d *defaultStore) CreatePreorderData(ctx context.Context, data PreorderFormData) error {
	if data.DataTrackingId == "" {
		return fmt.Errorf("data tracking id is required")
	}

	d.logger.Debugw("inserting preorder data", "dataTrackingId", data.DataTrackingId)

	res, err := d.preorderData.InsertOne(ctx, data)
	if err != nil {
		return err
	}

	d.logger.Infow(
		"Successfully inserted preorder data",
		"dataTrackingId", data.DataTrackingId,
		"_id", res.InsertedID,
	)

	return nil
}

func (d *defaultStore) CreateOrder(ctx context.Context, order OrderEvent) (*OrderEvent, error) {
	logger := d.logger.With(
		"deploymentId", order.OrderData.OrderInfo.Deployment,
		"orderId", order.OrderData.OrderInfo.OrderId,
	)

	logger.Debug("inserting xealth order event")

	res, err := d.orders.InsertOne(ctx, order)
	if err != nil {
		return nil, err
	}

	d.logger.Infow("Successfully inserted order event", "_id", res.InsertedID)

	return d.GetOrder(ctx, res.InsertedID.(primitive.ObjectID).Hex())
}

func (d *defaultStore) GetOrder(ctx context.Context, id string) (*OrderEvent, error) {
	objId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errs.NotFound
	}

	orderEvent := &OrderEvent{}
	err = d.orders.FindOne(ctx, bson.M{"_id": objId}).Decode(orderEvent)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, errs.NotFound
	} else if err != nil {
		return nil, err
	}

	return orderEvent, nil
}

func (d *defaultStore) GetReportView(ctx context.Context, documentId string) (*ReportView, error) {
	objId, err := primitive.ObjectIDFromHex(documentId)
	if err != nil {
		return nil, errs.NotFound
	}

	view := &ReportView{}
	err = d.reportViews.FindOne(ctx, bson.M{"_id": objId}).Decode(view)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, errs.NotFound
	} else if err != nil {
		return nil, err
	}

	return view, nil
}

func (d *defaultStore) GetMostRecentReportView(ctx context.Context, filter ReportViewFilter) (*ReportView, error) {
	opts := options.Find().SetSort(bson.M{"createdTime": -1}).SetLimit(1)
	cur, err := d.reportViews.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}

	var reports []ReportView
	if err := cur.All(ctx, &reports); err != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, errs.NotFound
	}

	return &reports[0], nil
}

func (d *defaultStore) CreateReportView(ctx context.Context, view ReportView) (*ReportView, error) {
	if view.UserId == "" {
		return nil, fmt.Errorf("userId is required")
	}
	if view.DeploymentId == "" {
		return nil, fmt.Errorf("deploymentId is required")
	}
	if view.ProgramId == "" {
		return nil, fmt.Errorf("programId is required")
	}
	if view.PatientUserId == "" {
		return nil, fmt.Errorf("patientUserId is required")
	}
	if view.ClinicId.IsZero() {
		return nil, fmt.Errorf("clinicId is required")
	}

	logger := d.logger.With("deploymentId", view.DeploymentId)
	logger.Debug("inserting report view")

	res, err := d.reportViews.InsertOne(ctx, view)
	if err != nil {
		return nil, err
	}

	d.logger.Infow("Successfully inserted report view", "_id", res.InsertedID)

	return d.GetReportView(ctx, res.InsertedID.(primitive.ObjectID).Hex())
}
