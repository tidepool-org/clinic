package patients

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	DeletionsCollectionName = "patient_deletions"
)

//go:generate mockgen --build_flags=--mod=mod -source=./deletions_repo.go -destination=./test/mock_deletions_repository.go -package test -aux_files=github.com/tidepool-org/clinic/patients=patients.go MockDeletionsRepository

type DeletionsRepository interface {
	Create(context.Context, Deletion) error
}


func NewDeletionsRepository(db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (DeletionsRepository, error) {
	repo := &deletionsRepository{
		collection: db.Collection(DeletionsCollectionName),
		logger:     logger,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return repo.Initialize(ctx)
		},
	})

	return repo, nil
}

type deletionsRepository struct {
	collection *mongo.Collection
	logger     *zap.SugaredLogger
}

func (p *deletionsRepository) Initialize(ctx context.Context) error {
	_, err := p.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "patient.clinicId", Value: 1},
				{Key: "patient.userId", Value: 1},
			},
			Options: options.Index().
				SetName("PatientDeletion"),
		},
		{
			Keys: bson.D{
				{Key: "deletedTime", Value: 1},
				{Key: "patient.clinicId", Value: 1},
			},
			Options: options.Index().
				SetName("DeletedTime"),
		},
	})
	return err
}

func (p *deletionsRepository) Create(ctx context.Context, deletion Deletion) error {
	if deletion.DeletedTime.IsZero() {
		return fmt.Errorf("deleted time cannot be zero")
	}
	if deletion.Patient.UserId == nil {
		return fmt.Errorf("patient user id cannot be nil")
	}
	if _, err := p.collection.InsertOne(ctx, deletion); err != nil {
		return fmt.Errorf("error persisting deleted patient: %w", err)
	}
	return nil
}
