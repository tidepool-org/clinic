// Package postgres mirrors clinician documents to PostgreSQL.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinicians"
	storepg "github.com/tidepool-org/clinic/store/postgres"
	"github.com/tidepool-org/clinic/store/postgres/sqlcgen"
)

func NewWriter(client *storepg.Client, logger *zap.SugaredLogger) *Writer {
	w := &Writer{client: client, logger: logger}
	if client.Enabled() {
		w.queries = sqlcgen.New(client.Pool())
	}
	return w
}

type Writer struct {
	client  *storepg.Client
	queries *sqlcgen.Queries
	logger  *zap.SugaredLogger
}

func (w *Writer) Enabled() bool {
	return w.queries != nil
}

// UpsertClinician snapshots a clinician row and replaces the roles-updates
// audit trail in a single transaction.
func (w *Writer) UpsertClinician(ctx context.Context, clinician *clinicians.Clinician) error {
	if clinician.Id == nil {
		return fmt.Errorf("clinician has no id")
	}
	if clinician.ClinicId == nil {
		return fmt.Errorf("clinician %s has no clinic id", clinician.Id.Hex())
	}
	id := clinician.Id.Hex()

	roles := clinician.Roles
	if roles == nil {
		roles = []string{}
	}

	tx, err := w.client.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := w.queries.WithTx(tx)
	if err := q.UpsertClinician(ctx, sqlcgen.UpsertClinicianParams{
		ID:               id,
		ClinicID:         clinician.ClinicId.Hex(),
		UserID:           textValue(clinician.UserId),
		Email:            textValue(clinician.Email),
		FullName:         textValue(clinician.Name),
		InviteID:         textValue(clinician.InviteId),
		Roles:            roles,
		IsServiceAccount: clinician.IsServiceAccount,
		CreatedTime:      pgtype.Timestamptz{Time: clinician.CreatedTime.UTC(), Valid: !clinician.CreatedTime.IsZero()},
		UpdatedTime:      pgtype.Timestamptz{Time: clinician.UpdatedTime.UTC(), Valid: !clinician.UpdatedTime.IsZero()},
	}); err != nil {
		return err
	}

	if err := q.DeleteClinicianRolesUpdates(ctx, id); err != nil {
		return err
	}
	for i, update := range clinician.RolesUpdates {
		updateRoles := update.Roles
		if updateRoles == nil {
			updateRoles = []string{}
		}
		if err := q.InsertClinicianRolesUpdate(ctx, sqlcgen.InsertClinicianRolesUpdateParams{
			ClinicianID: id,
			Ordinal:     int32(i),
			Roles:       updateRoles,
			UpdatedBy:   pgtype.Text{String: update.UpdatedBy, Valid: true},
		}); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (w *Writer) DeleteClinician(ctx context.Context, clinicId, userId string) error {
	return w.queries.DeleteClinician(ctx, sqlcgen.DeleteClinicianParams{
		ClinicID: clinicId,
		UserID:   pgtype.Text{String: userId, Valid: true},
	})
}

func (w *Writer) DeleteAllClinicians(ctx context.Context, clinicId string) error {
	return w.queries.DeleteAllClinicians(ctx, clinicId)
}

func (w *Writer) DeleteClinicianInvite(ctx context.Context, clinicId, inviteId string) error {
	return w.queries.DeleteClinicianInvite(ctx, sqlcgen.DeleteClinicianInviteParams{
		ClinicID: clinicId,
		InviteID: pgtype.Text{String: inviteId, Valid: true},
	})
}

func textValue(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

// NewBackfiller copies the clinicians collection using the same snapshot
// upserts as the dual-write path.
func NewBackfiller(db *mongo.Database, writer *Writer) storepg.Verifier {
	return storepg.NewCollectionSync(db.Collection(clinicians.CollectionName), "clinicians", func(ctx context.Context, docs []bson.M) error {
		for _, doc := range docs {
			raw, err := bson.Marshal(doc)
			if err != nil {
				return err
			}
			clinician := &clinicians.Clinician{}
			if err := bson.Unmarshal(raw, clinician); err != nil {
				return err
			}
			if err := writer.UpsertClinician(ctx, clinician); err != nil {
				return err
			}
		}
		return nil
	})
}
