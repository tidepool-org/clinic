// Package postgres mirrors redox documents to PostgreSQL.
//
// EHR messages are mirrored live: the redox handler invokes the Mirror after
// every successful Mongo insert. Rescheduled summary and reports orders are
// NOT mirrored live - the only writes to that collection happen inside the
// $merge aggregation pipeline in patients/repository (reschedulePipeline),
// which executes entirely server-side in Mongo and cannot be intercepted.
// The scheduled orders table converges through the pgsync backfiller instead,
// and `pgsync prune` replaces the Mongo TTL index.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/redox"
	models "github.com/tidepool-org/clinic/redox_models"
	"github.com/tidepool-org/clinic/store/dualwrite"
	storepg "github.com/tidepool-org/clinic/store/postgres"
	"github.com/tidepool-org/clinic/store/postgres/sqlcgen"
)

const (
	messagesCollection        = "redox"
	scheduledOrdersCollection = "scheduledSummaryAndReportsOrders"
)

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

func (w *Writer) UpsertMessage(ctx context.Context, envelope *models.MessageEnvelope) error {
	if envelope.Id.IsZero() {
		return fmt.Errorf("message envelope has no id")
	}

	payload, err := marshalPayload(envelope)
	if err != nil {
		return err
	}

	params := sqlcgen.UpsertRedoxMessageParams{
		ID:            envelope.Id.Hex(),
		MetaDataModel: envelope.Meta.DataModel,
		MetaEventType: envelope.Meta.EventType,
		MetaLogIds:    []string{},
		Payload:       payload,
	}
	if envelope.Meta.Source != nil {
		if envelope.Meta.Source.ID != nil {
			params.MetaSourceID = pgtype.Text{String: *envelope.Meta.Source.ID, Valid: true}
		}
		if envelope.Meta.Source.Name != nil {
			params.MetaSourceName = pgtype.Text{String: *envelope.Meta.Source.Name, Valid: true}
		}
	}
	if envelope.Meta.FacilityCode != nil {
		params.MetaFacilityCode = pgtype.Text{String: *envelope.Meta.FacilityCode, Valid: true}
	}
	if envelope.Meta.Logs != nil {
		for _, log := range *envelope.Meta.Logs {
			if log.ID != nil {
				params.MetaLogIds = append(params.MetaLogIds, *log.ID)
			}
		}
	}

	return w.queries.UpsertRedoxMessage(ctx, params)
}

// UpsertScheduledOrder mirrors a rescheduled summary and reports order. The
// documents are produced by an aggregation pipeline and have no domain
// struct, so the writer works with the raw document.
func (w *Writer) UpsertScheduledOrder(ctx context.Context, doc bson.M) error {
	id, ok := doc["_id"].(primitive.ObjectID)
	if !ok {
		return fmt.Errorf("scheduled order has no id")
	}

	normalized, err := storepg.NormalizeDocument(doc)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return err
	}

	params := sqlcgen.UpsertScheduledSummaryReportsOrderParams{
		ID:      id.Hex(),
		Payload: payload,
	}
	if clinicId, ok := doc["clinicId"].(primitive.ObjectID); ok {
		params.ClinicID = pgtype.Text{String: clinicId.Hex(), Valid: true}
	}
	if userId, ok := doc["userId"].(string); ok {
		params.UserID = pgtype.Text{String: userId, Valid: true}
	}
	if lastMatchedOrder, ok := doc["lastMatchedOrder"].(bson.M); ok {
		if orderId, ok := lastMatchedOrder["_id"].(primitive.ObjectID); ok {
			params.LastMatchedOrderID = pgtype.Text{String: orderId.Hex(), Valid: true}
		}
	}
	if createdTime, ok := doc["createdTime"].(primitive.DateTime); ok {
		params.CreatedTime = pgtype.Timestamptz{Time: createdTime.Time().UTC(), Valid: true}
	} else if createdTime, ok := doc["createdTime"].(time.Time); ok {
		params.CreatedTime = pgtype.Timestamptz{Time: createdTime.UTC(), Valid: true}
	} else {
		return fmt.Errorf("scheduled order %s has no created time", id.Hex())
	}

	return w.queries.UpsertScheduledSummaryReportsOrder(ctx, params)
}

// PruneScheduledOrders deletes mirrored scheduled orders older than the
// retention period, replacing the Mongo TTL index.
func (w *Writer) PruneScheduledOrders(ctx context.Context) (int64, error) {
	cutoff := time.Now().UTC().Add(-redox.RescheduledMessagesExpiration)
	return w.queries.PruneScheduledSummaryReportsOrders(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
}

func marshalPayload(v interface{}) ([]byte, error) {
	normalized, err := storepg.NormalizeDocument(v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

// NewMirror adapts the writer to the redox.Mirror interface consumed by the
// message handler. Mirroring is best-effort via dualwrite.Execute.
func NewMirror(client *storepg.Client, logger *zap.SugaredLogger) redox.Mirror {
	return &mirror{
		writer: NewWriter(client, logger),
		logger: logger,
	}
}

type mirror struct {
	writer *Writer
	logger *zap.SugaredLogger
}

func (m *mirror) CreateMessage(ctx context.Context, envelope models.MessageEnvelope) {
	if !m.writer.Enabled() {
		return
	}
	dualwrite.Execute(ctx, m.logger, "redox_messages", "create", func(ctx context.Context) error {
		return m.writer.UpsertMessage(ctx, &envelope)
	})
}

func NewMessageBackfiller(db *mongo.Database, writer *Writer) storepg.Verifier {
	return storepg.NewCollectionSync(db.Collection(messagesCollection), "redox_messages", func(ctx context.Context, docs []bson.M) error {
		for _, doc := range docs {
			raw, err := bson.Marshal(doc)
			if err != nil {
				return err
			}
			envelope := &models.MessageEnvelope{}
			if err := bson.Unmarshal(raw, envelope); err != nil {
				return err
			}
			if err := writer.UpsertMessage(ctx, envelope); err != nil {
				return err
			}
		}
		return nil
	})
}

func NewScheduledOrderBackfiller(db *mongo.Database, writer *Writer) storepg.Verifier {
	return storepg.NewCollectionSync(db.Collection(scheduledOrdersCollection), "scheduled_summary_reports_orders", func(ctx context.Context, docs []bson.M) error {
		for _, doc := range docs {
			if err := writer.UpsertScheduledOrder(ctx, doc); err != nil {
				return err
			}
		}
		return nil
	})
}
