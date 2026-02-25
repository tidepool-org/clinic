package outbox

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type repository struct {
	collection *mongo.Collection
	logger     *zap.SugaredLogger
}

func NewRepository(db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (Repository, error) {
	repo := &repository{
		collection: db.Collection(CollectionName),
		logger:     logger,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return repo.Initialize(ctx)
		},
	})

	return repo, nil
}

func (r *repository) Initialize(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "createdTime", Value: 1}},
			Options: options.Index().SetName("CreatedTime"),
		},
		{
			Keys:    bson.D{{Key: "eventType", Value: 1}},
			Options: options.Index().SetName("EventType"),
		},
	})
	return err
}

func (r *repository) Create(ctx context.Context, event Event) error {
	if _, err := r.collection.InsertOne(ctx, event); err != nil {
		return fmt.Errorf("error inserting outbox event: %w", err)
	}
	return nil
}
