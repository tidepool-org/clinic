package audit

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	collectionName = "clinician_audit_events"
)

type repository struct {
	collection *mongo.Collection
	logger     *zap.SugaredLogger
}

func NewRepository(db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (AuditEventRecorder, error) {
	repo := &repository{
		collection: db.Collection(collectionName),
		logger:     logger,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := repo.Initialize(ctx); err != nil {
				return err
			}
			return nil
		},
	})

	return repo, nil
}

func (r *repository) Initialize(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{"clinicId", 1},
				{"createdTime", 1},
			},
		},
		{
			Keys: bson.D{
				{"clinicianId", 1},
				{"createdTime", 1},
			},
		},
	})
	return err
}

func (r *repository) Create(ctx context.Context, event AuditEvent) error {
	event.CreatedTime = time.Now()
	_, err := r.collection.InsertOne(ctx, event)
	if err != nil {
		return fmt.Errorf("error creating audit event: %w", err)
	}
	return nil
}
