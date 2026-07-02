package merge

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// PlansRepository persists executed merge plans for auditing. There is no
// read path; plans are only ever inserted.
type PlansRepository interface {
	Persist(ctx context.Context, plan PlanMetadata) error
}

// PlanMetadata exposes the identity of a persisted plan document without the
// generic type parameter of PersistentPlan.
type PlanMetadata interface {
	PlanDocumentId() string
	PlanGroupId() string
	PlanType() string
	PlanPayload() any
}

func NewPlansRepository(db *mongo.Database, logger *zap.SugaredLogger) PlansRepository {
	return &plansRepository{
		collection: db.Collection(plansCollectionName),
	}
}

type plansRepository struct {
	collection *mongo.Collection
}

func (r *plansRepository) Persist(ctx context.Context, plan PlanMetadata) error {
	_, err := r.collection.InsertOne(ctx, plan)
	return err
}
