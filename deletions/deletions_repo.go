package deletions

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"time"
)

type Metadata struct {
	DeletedByUserId *string `bson:"deletedByUserId,omitempty"`
}

type Repository[T any] interface {
	Create(context.Context, T, Metadata) error
	CreateMany(context.Context, []T, Metadata) error
	Initialize(ctx context.Context, primaryKeyAttributes []string) error
}

func NewRepositoryFactory[T any](typ string, primaryKeyAttributes []string) func(db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (Repository[T], error) {
	return func(db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (Repository[T], error) {
		repo := &deletionsRepository[T]{
			collection: db.Collection(fmt.Sprintf("%s_deletions", typ)),
			logger:     logger,
			documentType: typ,
		}

		lifecycle.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				return repo.Initialize(ctx, primaryKeyAttributes)
			},
		})

		return repo, nil
	}
}

func NewRepository[T any](typ string, db *mongo.Database, logger *zap.SugaredLogger) (Repository[T], error) {
		repo := &deletionsRepository[T]{
			collection: db.Collection(fmt.Sprintf("%s_deletions", typ)),
			logger:     logger,
			documentType: typ,
		}

		return repo, nil
}

type deletionsRepository[T any] struct {
	collection   *mongo.Collection
	logger       *zap.SugaredLogger
	documentType string
}

func (p *deletionsRepository[T]) Initialize(ctx context.Context, primaryKeyAttributes []string) error {
	_, err := p.collection.Indexes().CreateMany(ctx, p.getIndexes(primaryKeyAttributes))
	return err
}

func (p *deletionsRepository[T]) getIndexes(primaryKeyAttributes []string) []mongo.IndexModel {
	var primaryIndexKeys bson.D

	for _, attr := range primaryKeyAttributes {
		primaryIndexKeys = append(primaryIndexKeys, primitive.E{
			Key:   fmt.Sprintf("%s.%s", p.documentType, attr),
			Value: 1,
		})
	}

	return []mongo.IndexModel{
		{
			Keys:    primaryIndexKeys,
			Options: options.Index().SetName(fmt.Sprintf("%sDeletion", cases.Title(language.English).String(p.documentType))),
		},
		{
			Keys:    append(bson.D{primitive.E{Key: "deletedTime", Value: 1}}, primaryIndexKeys...),
			Options: options.Index().SetName("DeletedTime"),
		},
	}
}

func (p *deletionsRepository[T]) Create(ctx context.Context, deleted T, meta Metadata) error {
	document := p.prepareDocument(deleted, meta)
	if _, err := p.collection.InsertOne(ctx, document); err != nil {
		return fmt.Errorf("error persisting deleted object in collection %s: %w", p.collection.Name(), err)
	}
	return nil
}

func (p *deletionsRepository[T]) CreateMany(ctx context.Context, deleted []T, meta Metadata) error {
	documents := make([]interface{}, 0, len(deleted))
	for _, d := range deleted {
		documents = append(documents, p.prepareDocument(d, meta))
	}

	if _, err := p.collection.InsertMany(ctx, documents); err != nil {
		return fmt.Errorf("error persisting deleted objects in collection %s: %w", p.collection.Name(), err)
	}
	return nil
}

func (p *deletionsRepository[T]) prepareDocument(deleted T, meta Metadata) bson.M {
	deletion := bson.M{
		"deletedTime":   time.Now(),
		p.documentType: deleted,
	}
	if meta.DeletedByUserId != nil {
		deletion["deletedByUserId"] = meta.DeletedByUserId
	}
	return deletion
}
