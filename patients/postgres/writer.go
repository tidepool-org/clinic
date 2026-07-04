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
	"unicode"

	"github.com/jackc/pgx/v5/pgtype"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/patients/postgres/sqlcgen"
	storepg "github.com/tidepool-org/clinic/store/postgres"
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
	// A stale row (e.g. a lost delete mirror for a patient later re-created
	// with a new id) would collide with UNIQUE(clinic_id, user_id) and wedge
	// every subsequent mirror write; Mongo enforces the same uniqueness, so
	// removing it converges the mirror.
	if err := q.DeleteConflictingPatients(ctx, sqlcgen.DeleteConflictingPatientsParams{
		ClinicID: params.ClinicID,
		UserID:   params.UserID,
		ID:       id,
	}); err != nil {
		return err
	}
	if err := q.UpsertPatient(ctx, *params); err != nil {
		return err
	}

	if err := q.DeletePatientTags(ctx, id); err != nil {
		return err
	}
	if patient.Tags != nil {
		tags := make([]sqlcgen.InsertPatientTagParams, 0, len(*patient.Tags))
		for _, tagId := range *patient.Tags {
			tags = append(tags, sqlcgen.InsertPatientTagParams{
				PatientID: id,
				TagID:     tagId.Hex(),
			})
		}
		if len(tags) > 0 {
			if err := storepg.ExecBatch(q.InsertPatientTag(ctx, tags)); err != nil {
				return err
			}
		}
	}

	if err := q.DeletePatientSites(ctx, id); err != nil {
		return err
	}
	if patient.Sites != nil {
		sites := make([]sqlcgen.InsertPatientSiteParams, 0, len(*patient.Sites))
		for _, site := range *patient.Sites {
			sites = append(sites, sqlcgen.InsertPatientSiteParams{
				PatientID: id,
				SiteID:    site.Id.Hex(),
				SiteName:  site.Name,
			})
		}
		if len(sites) > 0 {
			if err := storepg.ExecBatch(q.InsertPatientSite(ctx, sites)); err != nil {
				return err
			}
		}
	}

	if err := q.DeletePatientDataSources(ctx, id); err != nil {
		return err
	}
	if patient.DataSources != nil {
		dataSources := make([]sqlcgen.InsertPatientDataSourceParams, 0, len(*patient.DataSources))
		for i, source := range *patient.DataSources {
			params := sqlcgen.InsertPatientDataSourceParams{
				PatientID:      id,
				Ordinal:        int32(i),
				ProviderName:   source.ProviderName,
				State:          source.State,
				ModifiedTime:   storepg.TimestamptzValue(source.ModifiedTime),
				ExpirationTime: storepg.TimestamptzValue(source.ExpirationTime),
				LatestDataTime: storepg.TimestamptzValue(source.LatestDataTime),
			}
			if source.DataSourceId != nil {
				params.DataSourceID = pgtype.Text{String: source.DataSourceId.Hex(), Valid: true}
			}
			dataSources = append(dataSources, params)
		}
		if len(dataSources) > 0 {
			if err := storepg.ExecBatch(q.InsertPatientDataSource(ctx, dataSources)); err != nil {
				return err
			}
		}
	}

	if err := q.DeletePatientReviews(ctx, id); err != nil {
		return err
	}
	reviews := make([]sqlcgen.InsertPatientReviewParams, 0, len(patient.Reviews))
	for i, review := range patient.Reviews {
		reviews = append(reviews, sqlcgen.InsertPatientReviewParams{
			PatientID:   id,
			Ordinal:     int32(i),
			ClinicianID: review.ClinicianId,
			ReviewTime:  pgtype.Timestamptz{Time: review.Time.UTC(), Valid: true},
		})
	}
	if len(reviews) > 0 {
		if err := storepg.ExecBatch(q.InsertPatientReview(ctx, reviews)); err != nil {
			return err
		}
	}

	if err := q.DeletePatientConnectionRequests(ctx, id); err != nil {
		return err
	}
	var connectionRequests []sqlcgen.InsertPatientConnectionRequestParams
	for _, requests := range patient.ProviderConnectionRequests {
		for _, request := range requests {
			connectionRequests = append(connectionRequests, sqlcgen.InsertPatientConnectionRequestParams{
				PatientID:    id,
				ProviderName: request.ProviderName,
				CreatedTime:  pgtype.Timestamptz{Time: request.CreatedTime.UTC(), Valid: true},
			})
		}
	}
	if len(connectionRequests) > 0 {
		if err := storepg.ExecBatch(q.InsertPatientConnectionRequest(ctx, connectionRequests)); err != nil {
			return err
		}
	}

	if err := upsertSummaries(ctx, q, id, patient.Summary); err != nil {
		return err
	}

	// Matched messages cascade when their subscription rows are deleted
	if err := q.DeletePatientEHRSubscriptions(ctx, id); err != nil {
		return err
	}
	subscriptions := make([]sqlcgen.InsertPatientEHRSubscriptionParams, 0, len(patient.EHRSubscriptions))
	var matchedMessages []sqlcgen.InsertPatientEHRSubscriptionMatchedMessageParams
	for name, subscription := range patient.EHRSubscriptions {
		subscriptions = append(subscriptions, sqlcgen.InsertPatientEHRSubscriptionParams{
			PatientID:   id,
			Name:        name,
			Provider:    subscription.Provider,
			Active:      subscription.Active,
			CreatedTime: pgtype.Timestamptz{Time: subscription.CreatedAt.UTC(), Valid: !subscription.CreatedAt.IsZero()},
			UpdatedTime: pgtype.Timestamptz{Time: subscription.UpdatedAt.UTC(), Valid: !subscription.UpdatedAt.IsZero()},
		})
		for i, message := range subscription.MatchedMessages {
			matchedMessages = append(matchedMessages, sqlcgen.InsertPatientEHRSubscriptionMatchedMessageParams{
				PatientID:        id,
				SubscriptionName: name,
				Ordinal:          int32(i),
				MessageID:        message.DocumentId.Hex(),
				DataModel:        message.DataModel,
				EventType:        message.EventType,
			})
		}
	}
	if len(subscriptions) > 0 {
		if err := storepg.ExecBatch(q.InsertPatientEHRSubscription(ctx, subscriptions)); err != nil {
			return err
		}
	}
	// Matched-message rows require their subscription rows to exist first
	if len(matchedMessages) > 0 {
		if err := storepg.ExecBatch(q.InsertPatientEHRSubscriptionMatchedMessage(ctx, matchedMessages)); err != nil {
			return err
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
		FullName:               storepg.TextValue(patient.FullName),
		BirthDate:              storepg.TextValue(patient.BirthDate),
		Email:                  storepg.TextValue(patient.Email),
		Mrn:                    storepg.TextValue(patient.Mrn),
		RequireUniqueMrn:       patient.RequireUniqueMrn,
		IsMigrated:             patient.IsMigrated,
		InvitedBy:              storepg.TextValue(patient.InvitedBy),
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
		params.GlycemicRangesType = storepg.NonEmptyTextValue(string(ranges.Type))
		params.GlycemicRangesPreset = storepg.NonEmptyTextValue(string(ranges.Preset))
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

// NewBackfiller copies the patients collection using the same snapshot
// upserts as the dual-write path.
func NewBackfiller(db *mongo.Database, writer *Writer) storepg.Verifier {
	return storepg.NewCollectionSync(db.Collection(patients.CollectionName), "patients",
		storepg.UnmarshalAndUpsert(writer.UpsertPatient))
}
