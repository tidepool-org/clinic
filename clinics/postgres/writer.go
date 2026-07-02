// Package postgres mirrors clinic documents to PostgreSQL. The clinic
// aggregate is fully denormalized: every embedded document used for
// filtering lives in a real column or child table.
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

	"github.com/tidepool-org/clinic/clinics"
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

// UpsertClinic snapshots the clinic into the clinics row and replaces all
// child sets (share codes, admins, phone numbers, membership restrictions,
// patient tags, sites) in a single transaction, so dual writes, retries and
// backfill all converge to the latest Mongo state.
func (w *Writer) UpsertClinic(ctx context.Context, clinic *clinics.Clinic) error {
	if clinic.Id == nil {
		return fmt.Errorf("clinic has no id")
	}
	id := clinic.Id.Hex()

	params, err := upsertClinicParams(id, clinic)
	if err != nil {
		return err
	}

	tx, err := w.client.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := w.queries.WithTx(tx)
	if err := q.UpsertClinic(ctx, *params); err != nil {
		return err
	}

	if err := q.DeleteClinicShareCodes(ctx, id); err != nil {
		return err
	}
	if clinic.ShareCodes != nil {
		for _, shareCode := range *clinic.ShareCodes {
			if err := q.InsertClinicShareCode(ctx, sqlcgen.InsertClinicShareCodeParams{
				ShareCode: shareCode,
				ClinicID:  id,
			}); err != nil {
				return err
			}
		}
	}

	if err := q.DeleteClinicAdmins(ctx, id); err != nil {
		return err
	}
	if clinic.Admins != nil {
		for _, userId := range *clinic.Admins {
			if err := q.InsertClinicAdmin(ctx, sqlcgen.InsertClinicAdminParams{
				ClinicID: id,
				UserID:   userId,
			}); err != nil {
				return err
			}
		}
	}

	if err := q.DeleteClinicPhoneNumbers(ctx, id); err != nil {
		return err
	}
	if clinic.PhoneNumbers != nil {
		for i, phoneNumber := range *clinic.PhoneNumbers {
			if err := q.InsertClinicPhoneNumber(ctx, sqlcgen.InsertClinicPhoneNumberParams{
				ClinicID: id,
				Ordinal:  int32(i),
				Type:     textValue(phoneNumber.Type),
				Number:   phoneNumber.Number,
			}); err != nil {
				return err
			}
		}
	}

	if err := q.DeleteClinicMembershipRestrictions(ctx, id); err != nil {
		return err
	}
	for _, restriction := range clinic.MembershipRestrictions {
		if err := q.InsertClinicMembershipRestriction(ctx, sqlcgen.InsertClinicMembershipRestrictionParams{
			ClinicID:    id,
			EmailDomain: restriction.EmailDomain,
			RequiredIdp: pgtype.Text{String: restriction.RequiredIdp, Valid: true},
		}); err != nil {
			return err
		}
	}

	if err := q.DeleteClinicPatientTags(ctx, id); err != nil {
		return err
	}
	for _, tag := range clinic.PatientTags {
		if tag.Id == nil {
			return fmt.Errorf("patient tag of clinic %s has no id", id)
		}
		if err := q.InsertClinicPatientTag(ctx, sqlcgen.InsertClinicPatientTagParams{
			ID:       tag.Id.Hex(),
			ClinicID: id,
			Name:     tag.Name,
		}); err != nil {
			return err
		}
	}

	if err := q.DeleteClinicSites(ctx, id); err != nil {
		return err
	}
	for _, site := range clinic.Sites {
		if err := q.InsertClinicSite(ctx, sqlcgen.InsertClinicSiteParams{
			ID:       site.Id.Hex(),
			ClinicID: id,
			Name:     site.Name,
		}); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (w *Writer) DeleteClinic(ctx context.Context, clinicId string) error {
	return w.queries.DeleteClinic(ctx, clinicId)
}

func upsertClinicParams(id string, clinic *clinics.Clinic) (*sqlcgen.UpsertClinicParams, error) {
	params := &sqlcgen.UpsertClinicParams{
		ID:                 id,
		Name:               textValue(clinic.Name),
		Address:            textValue(clinic.Address),
		City:               textValue(clinic.City),
		State:              textValue(clinic.State),
		PostalCode:         textValue(clinic.PostalCode),
		Country:            textValue(clinic.Country),
		ClinicType:         textValue(clinic.ClinicType),
		ClinicSize:         textValue(clinic.ClinicSize),
		Website:            textValue(clinic.Website),
		Timezone:           textValue(clinic.Timezone),
		PreferredBgUnits:   nonEmptyTextValue(clinic.PreferredBgUnits),
		CanonicalShareCode: textValue(clinic.CanonicalShareCode),
		Tier:               nonEmptyTextValue(clinic.Tier),
		IsMigrated:         clinic.IsMigrated,
		CreatedTime:        pgtype.Timestamptz{Time: clinic.CreatedTime.UTC(), Valid: !clinic.CreatedTime.IsZero()},
		UpdatedTime:        pgtype.Timestamptz{Time: clinic.UpdatedTime.UTC(), Valid: !clinic.UpdatedTime.IsZero()},
	}

	if clinic.SuppressedNotifications != nil {
		params.SuppressPatientClinicInvitation = boolValue(clinic.SuppressedNotifications.PatientClinicInvitation)
	}

	if clinic.MRNSettings != nil {
		params.MrnRequired = pgtype.Bool{Bool: clinic.MRNSettings.Required, Valid: true}
		params.MrnUnique = pgtype.Bool{Bool: clinic.MRNSettings.Unique, Valid: true}
	}

	if ehr := clinic.EHRSettings; ehr != nil {
		params.EhrEnabled = pgtype.Bool{Bool: ehr.Enabled, Valid: true}
		params.EhrProvider = pgtype.Text{String: ehr.Provider, Valid: true}
		params.EhrSourceID = pgtype.Text{String: ehr.SourceId, Valid: true}
		params.EhrMrnIDType = pgtype.Text{String: ehr.MrnIdType, Valid: true}
		if ehr.DestinationIds != nil {
			params.EhrDestinationFlowsheet = pgtype.Text{String: ehr.DestinationIds.Flowsheet, Valid: true}
			params.EhrDestinationNotes = pgtype.Text{String: ehr.DestinationIds.Notes, Valid: true}
			params.EhrDestinationResults = pgtype.Text{String: ehr.DestinationIds.Results, Valid: true}
		}
		params.EhrProcedureEnableSummaryReports = textValue(ehr.ProcedureCodes.EnableSummaryReports)
		params.EhrProcedureDisableSummaryReports = textValue(ehr.ProcedureCodes.DisableSummaryReports)
		params.EhrProcedureCreateAccount = textValue(ehr.ProcedureCodes.CreateAccount)
		params.EhrProcedureCreateAccountAndEnableReports = textValue(ehr.ProcedureCodes.CreateAccountAndEnableReports)
		params.EhrScheduledReportsCadence = pgtype.Text{String: ehr.ScheduledReports.Cadence, Valid: true}
		params.EhrScheduledReportsOnUploadEnabled = pgtype.Bool{Bool: ehr.ScheduledReports.OnUploadEnabled, Valid: true}
		params.EhrScheduledReportsOnUploadNoteEventType = textValue(ehr.ScheduledReports.OnUploadNoteEventType)
		params.EhrTagsCodes = ehr.Tags.Codes
		params.EhrTagsSeparator = textValue(ehr.Tags.Separator)
		params.EhrFlowsheetsIcode = pgtype.Bool{Bool: ehr.Flowsheets.Icode, Valid: true}
		params.EhrNotesIncludeGmi = pgtype.Bool{Bool: ehr.Notes.IncludeGMI, Valid: true}
	}

	if pcs := clinic.PatientCountSettings; pcs != nil {
		if pcs.HardLimit != nil {
			params.PcsHardLimitPlan = pgtype.Int4{Int32: int32(pcs.HardLimit.Plan), Valid: true}
			params.PcsHardLimitStartDate = timestamptzValue(pcs.HardLimit.StartDate)
			params.PcsHardLimitEndDate = timestamptzValue(pcs.HardLimit.EndDate)
			params.PcsHardLimitLegacyPatientCount = intValue(pcs.HardLimit.PatientCount)
		}
		if pcs.SoftLimit != nil {
			params.PcsSoftLimitPlan = pgtype.Int4{Int32: int32(pcs.SoftLimit.Plan), Valid: true}
			params.PcsSoftLimitStartDate = timestamptzValue(pcs.SoftLimit.StartDate)
			params.PcsSoftLimitEndDate = timestamptzValue(pcs.SoftLimit.EndDate)
			params.PcsSoftLimitLegacyPatientCount = intValue(pcs.SoftLimit.PatientCount)
		}
	}

	if count := clinic.PatientCount; count != nil {
		params.PatientCountTotal = pgtype.Int4{Int32: int32(count.Total), Valid: true}
		params.PatientCountDemo = pgtype.Int4{Int32: int32(count.Demo), Valid: true}
		params.PatientCountPlan = pgtype.Int4{Int32: int32(count.Plan), Valid: true}
		params.PatientCountLegacy = intValue(count.PatientCount)
		if count.Providers != nil {
			providers, err := json.Marshal(count.Providers)
			if err != nil {
				return nil, fmt.Errorf("unable to marshal patient count providers: %w", err)
			}
			params.PatientCountProviders = providers
		}
	}

	return params, nil
}

func textValue(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func nonEmptyTextValue(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

func boolValue(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}

func intValue(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

func timestamptzValue(v *time.Time) pgtype.Timestamptz {
	if v == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: v.UTC(), Valid: true}
}

// NewBackfiller copies the clinics collection using the same snapshot
// upserts as the dual-write path.
func NewBackfiller(db *mongo.Database, writer *Writer) storepg.Backfiller {
	return &backfiller{
		collection: db.Collection(clinics.CollectionName),
		writer:     writer,
	}
}

type backfiller struct {
	collection *mongo.Collection
	writer     *Writer
}

func (b *backfiller) Collection() string {
	return b.collection.Name()
}

func (b *backfiller) BackfillBatch(ctx context.Context, after primitive.ObjectID, limit int) (primitive.ObjectID, int, error) {
	return storepg.BackfillDocuments(ctx, b.collection, after, limit, func(ctx context.Context, docs []bson.M) error {
		for _, doc := range docs {
			raw, err := bson.Marshal(doc)
			if err != nil {
				return err
			}
			clinic := &clinics.Clinic{}
			if err := bson.Unmarshal(raw, clinic); err != nil {
				return err
			}
			if err := b.writer.UpsertClinic(ctx, clinic); err != nil {
				return err
			}
		}
		return nil
	})
}
