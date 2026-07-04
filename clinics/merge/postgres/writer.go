// Package postgres mirrors executed merge plans to PostgreSQL.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics/merge"
	"github.com/tidepool-org/clinic/store/dualwrite"
	storepg "github.com/tidepool-org/clinic/store/postgres"
	"github.com/tidepool-org/clinic/clinics/merge/postgres/sqlcgen"
)

const collectionName = "merge_plans"

func NewWriter(client *storepg.Client, logger *zap.SugaredLogger) *Writer {
	w := &Writer{logger: logger}
	if client.Enabled() {
		w.queries = sqlcgen.New(client.Pool())
	}
	return w
}

type Writer struct {
	queries *sqlcgen.Queries
	logger  *zap.SugaredLogger
}

func (w *Writer) Enabled() bool {
	return w.queries != nil
}

func (w *Writer) UpsertPlan(ctx context.Context, id, planId, typ string, plan any) error {
	objId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid merge plan id %q: %w", id, err)
	}

	normalized, err := storepg.NormalizeDocument(bson.M{
		"plan":   plan,
		"planId": planId,
		"type":   typ,
	})
	if err != nil {
		return fmt.Errorf("error normalizing merge plan %s: %w", id, err)
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("error marshaling merge plan %s: %w", id, err)
	}

	return w.queries.UpsertMergePlan(ctx, sqlcgen.UpsertMergePlanParams{
		ID:          id,
		PlanID:      planId,
		Type:        typ,
		Payload:     payload,
		CreatedTime: pgtype.Timestamptz{Time: objId.Timestamp().UTC(), Valid: true},
	})
}

// NewDualPlansRepository decorates the Mongo merge plans repository with
// best-effort mirroring to PostgreSQL.
func NewDualPlansRepository(repo merge.PlansRepository, writer *Writer, logger *zap.SugaredLogger) merge.PlansRepository {
	if !writer.Enabled() {
		return repo
	}
	return &dualPlansRepository{
		repo:   repo,
		writer: writer,
		logger: logger,
	}
}

type dualPlansRepository struct {
	repo   merge.PlansRepository
	writer *Writer
	logger *zap.SugaredLogger
}

func (d *dualPlansRepository) Persist(ctx context.Context, plan merge.PlanMetadata) error {
	if err := d.repo.Persist(ctx, plan); err != nil {
		return err
	}
	id := plan.PlanDocumentId()
	planId := plan.PlanGroupId()
	typ := plan.PlanType()
	payload := plan.PlanPayload()
	dualwrite.Execute(ctx, d.logger, "merge_plans", "persist", func(ctx context.Context) error {
		return d.writer.UpsertPlan(ctx, id, planId, typ, payload)
	})
	return nil
}

// NewBackfiller backfills the merge plans collection.
func NewBackfiller(db *mongo.Database, writer *Writer) storepg.Verifier {
	return storepg.NewCollectionSync(db.Collection(collectionName), collectionName, func(ctx context.Context, docs []bson.M) error {
		for _, doc := range docs {
			id, ok := doc["_id"].(primitive.ObjectID)
			if !ok {
				return fmt.Errorf("merge plan has a non object id key")
			}
			planId, ok := doc["planId"].(primitive.ObjectID)
			if !ok {
				return fmt.Errorf("merge plan %s has a non object id planId", id.Hex())
			}
			typ, ok := doc["type"].(string)
			if !ok {
				return fmt.Errorf("merge plan %s has a non string type", id.Hex())
			}
			if err := writer.UpsertPlan(ctx, id.Hex(), planId.Hex(), typ, doc["plan"]); err != nil {
				return err
			}
		}
		return nil
	})
}
