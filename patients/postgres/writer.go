// Package postgres mirrors patient documents to PostgreSQL. The patient
// aggregate is fully denormalized: every embedded document used for
// filtering or sorting lives in a real column or child table. Summary
// statistics are mirrored separately.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgtype"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/tidepool-org/clinic/patients"
	storepg "github.com/tidepool-org/clinic/store/postgres"
	"github.com/tidepool-org/clinic/store/postgres/sqlcgen"
)

// NormalizeFullName lowercases and strips diacritics from a patient name.
// It replaces Mongo's strength-1 collation index for name search; queries
// must normalize their search terms the same way.
func NormalizeFullName(name string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	normalized, _, err := transform.String(t, name)
	if err != nil {
		normalized = name
	}
	return strings.ToLower(normalized)
}

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

// UpsertPatient snapshots the patient into the patients row and replaces all
// child sets (tags, sites, data sources, reviews, connection requests, EHR
// subscriptions) in a single transaction, so dual writes, retries and
// backfill all converge to the latest Mongo state.
func (w *Writer) UpsertPatient(ctx context.Context, patient *patients.Patient) error {
	if patient.Id == nil {
		return fmt.Errorf("patient has no id")
	}
	if patient.ClinicId == nil || patient.UserId == nil {
		return fmt.Errorf("patient %s has no clinic id or user id", patient.Id.Hex())
	}
	id := patient.Id.Hex()

	params, err := upsertPatientParams(id, patient)
	if err != nil {
		return err
	}

	tx, err := w.client.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := w.queries.WithTx(tx)
	if err := q.UpsertPatient(ctx, *params); err != nil {
		return err
	}

	if err := q.DeletePatientTags(ctx, id); err != nil {
		return err
	}
	if patient.Tags != nil {
		for _, tagId := range *patient.Tags {
			if err := q.InsertPatientTag(ctx, sqlcgen.InsertPatientTagParams{
				PatientID: id,
				TagID:     tagId.Hex(),
			}); err != nil {
				return err
			}
		}
	}

	if err := q.DeletePatientSites(ctx, id); err != nil {
		return err
	}
	if patient.Sites != nil {
		for _, site := range *patient.Sites {
			if err := q.InsertPatientSite(ctx, sqlcgen.InsertPatientSiteParams{
				PatientID: id,
				SiteID:    site.Id.Hex(),
				SiteName:  site.Name,
			}); err != nil {
				return err
			}
		}
	}

	if err := q.DeletePatientDataSources(ctx, id); err != nil {
		return err
	}
	if patient.DataSources != nil {
		for i, source := range *patient.DataSources {
			params := sqlcgen.InsertPatientDataSourceParams{
				PatientID:      id,
				Ordinal:        int32(i),
				ProviderName:   source.ProviderName,
				State:          source.State,
				ModifiedTime:   timestamptzValue(source.ModifiedTime),
				ExpirationTime: timestamptzValue(source.ExpirationTime),
				LatestDataTime: timestamptzValue(source.LatestDataTime),
			}
			if source.DataSourceId != nil {
				params.DataSourceID = pgtype.Text{String: source.DataSourceId.Hex(), Valid: true}
			}
			if err := q.InsertPatientDataSource(ctx, params); err != nil {
				return err
			}
		}
	}

	if err := q.DeletePatientReviews(ctx, id); err != nil {
		return err
	}
	for i, review := range patient.Reviews {
		if err := q.InsertPatientReview(ctx, sqlcgen.InsertPatientReviewParams{
			PatientID:   id,
			Ordinal:     int32(i),
			ClinicianID: review.ClinicianId,
			ReviewTime:  pgtype.Timestamptz{Time: review.Time.UTC(), Valid: true},
		}); err != nil {
			return err
		}
	}

	if err := q.DeletePatientConnectionRequests(ctx, id); err != nil {
		return err
	}
	for _, requests := range patient.ProviderConnectionRequests {
		for _, request := range requests {
			if err := q.InsertPatientConnectionRequest(ctx, sqlcgen.InsertPatientConnectionRequestParams{
				PatientID:    id,
				ProviderName: request.ProviderName,
				CreatedTime:  pgtype.Timestamptz{Time: request.CreatedTime.UTC(), Valid: true},
			}); err != nil {
				return err
			}
		}
	}

	// Matched messages cascade when their subscription rows are deleted
	if err := q.DeletePatientEHRSubscriptions(ctx, id); err != nil {
		return err
	}
	for name, subscription := range patient.EHRSubscriptions {
		if err := q.InsertPatientEHRSubscription(ctx, sqlcgen.InsertPatientEHRSubscriptionParams{
			PatientID:   id,
			Name:        name,
			Provider:    subscription.Provider,
			Active:      subscription.Active,
			CreatedTime: pgtype.Timestamptz{Time: subscription.CreatedAt.UTC(), Valid: !subscription.CreatedAt.IsZero()},
			UpdatedTime: pgtype.Timestamptz{Time: subscription.UpdatedAt.UTC(), Valid: !subscription.UpdatedAt.IsZero()},
		}); err != nil {
			return err
		}
		for i, message := range subscription.MatchedMessages {
			if err := q.InsertPatientEHRSubscriptionMatchedMessage(ctx, sqlcgen.InsertPatientEHRSubscriptionMatchedMessageParams{
				PatientID:        id,
				SubscriptionName: name,
				Ordinal:          int32(i),
				MessageID:        message.DocumentId.Hex(),
				DataModel:        message.DataModel,
				EventType:        message.EventType,
			}); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

func (w *Writer) DeletePatient(ctx context.Context, clinicId, userId string) error {
	return w.queries.DeletePatient(ctx, sqlcgen.DeletePatientParams{
		ClinicID: clinicId,
		UserID:   userId,
	})
}

func (w *Writer) DeletePatientsByUserId(ctx context.Context, userId string) error {
	return w.queries.DeletePatientsByUserId(ctx, userId)
}

func (w *Writer) DeleteNonCustodialPatientsOfClinic(ctx context.Context, clinicId string) error {
	return w.queries.DeleteNonCustodialPatientsOfClinic(ctx, clinicId)
}

// AssignTag mirrors the Mongo bulk tag assignment: a nil userIds slice
// targets every patient of the clinic.
func (w *Writer) AssignTag(ctx context.Context, clinicId, tagId string, userIds []string) error {
	if userIds == nil {
		return w.queries.AssignTagToClinicPatients(ctx, sqlcgen.AssignTagToClinicPatientsParams{
			ClinicID: clinicId,
			TagID:    tagId,
		})
	}
	return w.queries.AssignTagToPatients(ctx, sqlcgen.AssignTagToPatientsParams{
		ClinicID: clinicId,
		TagID:    tagId,
		UserIds:  userIds,
	})
}

// DeleteTag mirrors the Mongo bulk tag removal: a nil userIds slice targets
// every patient of the clinic.
func (w *Writer) DeleteTag(ctx context.Context, clinicId, tagId string, userIds []string) error {
	if userIds == nil {
		return w.queries.DeleteTagFromClinicPatients(ctx, sqlcgen.DeleteTagFromClinicPatientsParams{
			ClinicID: clinicId,
			TagID:    tagId,
		})
	}
	return w.queries.DeleteTagFromPatients(ctx, sqlcgen.DeleteTagFromPatientsParams{
		ClinicID: clinicId,
		TagID:    tagId,
		UserIds:  userIds,
	})
}

func (w *Writer) DeleteSite(ctx context.Context, clinicId, siteId string) error {
	return w.queries.DeleteSiteFromClinicPatients(ctx, sqlcgen.DeleteSiteFromClinicPatientsParams{
		ClinicID: clinicId,
		SiteID:   siteId,
	})
}

func (w *Writer) RenameSite(ctx context.Context, clinicId, siteId, name string) error {
	return w.queries.RenameSiteForClinicPatients(ctx, sqlcgen.RenameSiteForClinicPatientsParams{
		ClinicID: clinicId,
		SiteID:   siteId,
		SiteName: name,
	})
}

// MergeSites adds the target site to every patient assigned to the source
// site and removes the source site, mirroring the Mongo update pair.
func (w *Writer) MergeSites(ctx context.Context, clinicId, sourceSiteId, targetSiteId, targetSiteName string) error {
	tx, err := w.client.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := w.queries.WithTx(tx)
	if err := q.AddSiteToPatientsWithSite(ctx, sqlcgen.AddSiteToPatientsWithSiteParams{
		ClinicID:       clinicId,
		SourceSiteID:   sourceSiteId,
		TargetSiteID:   targetSiteId,
		TargetSiteName: targetSiteName,
	}); err != nil {
		return err
	}
	if err := q.DeleteSiteFromClinicPatients(ctx, sqlcgen.DeleteSiteFromClinicPatientsParams{
		ClinicID: clinicId,
		SiteID:   sourceSiteId,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ConvertTagToSite assigns the site to every patient carrying the tag and
// removes the tag, mirroring the Mongo update.
func (w *Writer) ConvertTagToSite(ctx context.Context, clinicId, tagId, siteId, siteName string) error {
	tx, err := w.client.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := w.queries.WithTx(tx)
	if err := q.AddSiteToPatientsWithTag(ctx, sqlcgen.AddSiteToPatientsWithTagParams{
		ClinicID: clinicId,
		TagID:    tagId,
		SiteID:   siteId,
		SiteName: siteName,
	}); err != nil {
		return err
	}
	if err := q.DeleteTagFromClinicPatients(ctx, sqlcgen.DeleteTagFromClinicPatientsParams{
		ClinicID: clinicId,
		TagID:    tagId,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func upsertPatientParams(id string, patient *patients.Patient) (*sqlcgen.UpsertPatientParams, error) {
	params := &sqlcgen.UpsertPatientParams{
		ID:                     id,
		ClinicID:               patient.ClinicId.Hex(),
		UserID:                 *patient.UserId,
		FullName:               textValue(patient.FullName),
		BirthDate:              textValue(patient.BirthDate),
		Email:                  textValue(patient.Email),
		Mrn:                    textValue(patient.Mrn),
		RequireUniqueMrn:       patient.RequireUniqueMrn,
		IsMigrated:             patient.IsMigrated,
		InvitedBy:              textValue(patient.InvitedBy),
		LegacyClinicianIds:     patient.LegacyClinicianIds,
		CreatedTime:            pgtype.Timestamptz{Time: patient.CreatedTime.UTC(), Valid: !patient.CreatedTime.IsZero()},
		UpdatedTime:            pgtype.Timestamptz{Time: patient.UpdatedTime.UTC(), Valid: !patient.UpdatedTime.IsZero()},
		LastUploadReminderTime: pgtype.Timestamptz{Time: patient.LastUploadReminderTime.UTC(), Valid: !patient.LastUploadReminderTime.IsZero()},
		LastRequestedDexcomConnectTime: pgtype.Timestamptz{
			Time:  patient.LastRequestedDexcomConnectTime.UTC(),
			Valid: !patient.LastRequestedDexcomConnectTime.IsZero(),
		},
	}

	if patient.FullName != nil {
		params.FullNameNormalized = pgtype.Text{String: NormalizeFullName(*patient.FullName), Valid: true}
	}
	if patient.TargetDevices != nil {
		params.TargetDevices = *patient.TargetDevices
	}
	if patient.DiagnosisType != nil {
		params.DiagnosisType = pgtype.Text{String: string(*patient.DiagnosisType), Valid: true}
	}

	if permissions := patient.Permissions; permissions != nil {
		params.PermCustodian = permissions.Custodian != nil
		params.PermView = permissions.View != nil
		params.PermUpload = permissions.Upload != nil
		params.PermNote = permissions.Note != nil
	}

	if ranges := patient.GlycemicRanges; !ranges.IsZero() {
		params.GlycemicRangesType = nonEmptyTextValue(string(ranges.Type))
		params.GlycemicRangesPreset = nonEmptyTextValue(string(ranges.Preset))
		if !ranges.Custom.IsZero() {
			custom, err := json.Marshal(ranges.Custom)
			if err != nil {
				return nil, fmt.Errorf("unable to marshal custom glycemic ranges: %w", err)
			}
			params.GlycemicRangesCustom = custom
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

func timestamptzValue(v *time.Time) pgtype.Timestamptz {
	if v == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: v.UTC(), Valid: true}
}

// NewBackfiller copies the patients collection using the same snapshot
// upserts as the dual-write path.
func NewBackfiller(db *mongo.Database, writer *Writer) storepg.Backfiller {
	return &backfiller{
		collection: db.Collection(patients.CollectionName),
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
			patient, err := unmarshalPatient(doc)
			if err != nil {
				return err
			}
			if err := b.writer.UpsertPatient(ctx, patient); err != nil {
				return err
			}
		}
		return nil
	})
}

// unmarshalPatient decodes a raw patient document honoring JSON struct tags,
// matching the store client's BSON options. A plain bson.Unmarshal would
// mis-map fields that only carry json tags, such as reviews' clinicianId.
func unmarshalPatient(doc bson.M) (*patients.Patient, error) {
	raw, err := bson.Marshal(doc)
	if err != nil {
		return nil, err
	}
	decoder, err := bson.NewDecoder(bsonrw.NewBSONDocumentReader(raw))
	if err != nil {
		return nil, err
	}
	decoder.UseJSONStructTags()
	patient := &patients.Patient{}
	if err := decoder.Decode(patient); err != nil {
		return nil, err
	}
	return patient, nil
}
