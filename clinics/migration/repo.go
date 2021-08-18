package migration

import (
	"context"
	"errors"
	"fmt"
	internalErrs "github.com/tidepool-org/clinic/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
)

const (
	migrationsCollectionName = "migrations"
)

var (
	ErrNotFound  = fmt.Errorf("migration %w", internalErrs.NotFound)
	ErrDuplicate = fmt.Errorf("migration %w", internalErrs.Duplicate)
)

type Repository interface {
	Get(ctx context.Context, clinicId, userId string) (*Migration, error)
	List(ctx context.Context, clinicId string) ([]*Migration, error)
	Create(ctx context.Context, migration *Migration) (*Migration, error)
}

func NewRepository(db *mongo.Database, lifecycle fx.Lifecycle) (Repository, error) {
	repo := &repository{
		collection: db.Collection(migrationsCollectionName),
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return repo.Initialize(ctx)
		},
	})

	return repo, nil
}

type repository struct {
	collection *mongo.Collection
}

func (r *repository) Initialize(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "userId", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetUnique(true).
				SetName("UniqueClinicianMigration"),
		},
	})
	return err
}

func (r *repository) Get(ctx context.Context, clinicId, userId string) (*Migration, error) {
	id, _ := primitive.ObjectIDFromHex(clinicId)
	return r.get(ctx, bson.M{"userId": userId, "clinicId": id})
}

func (r *repository) List(ctx context.Context, clinicId string) ([]*Migration, error) {
	id, _ := primitive.ObjectIDFromHex(clinicId)
	selector := bson.M{
		"clinicId": id,
	}
	cursor, err := r.collection.Find(ctx, selector)
	if err != nil {
		return nil, fmt.Errorf("error listing migrations: %w", err)
	}

	var migrations []*Migration
	if err = cursor.All(ctx, &migrations); err != nil {
		return nil, fmt.Errorf("error decoding migrations: %w", err)
	}

	return migrations, nil
}

func (r *repository) Create(ctx context.Context, migration *Migration) (*Migration, error) {
	_, err := r.get(ctx, bson.M{"userId": migration.UserId})
	if err == nil {
		return nil, ErrDuplicate
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	res, err := r.collection.InsertOne(ctx, migration)
	if err != nil {
		return nil, err
	}

	return r.get(ctx, bson.M{
		"_id": res.InsertedID,
	})
}

func (r *repository) get(ctx context.Context, selector bson.M) (*Migration, error) {
	result := &Migration{}
	err := r.collection.FindOne(ctx, selector).Decode(result)
	if err != nil && err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	}
	return result, err
}
