// Package postgres mirrors clinic documents to PostgreSQL. The clinic
// aggregate is fully denormalized: every embedded document used for
// filtering lives in a real column or child table.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/postgres/sqlcgen"
	storepg "github.com/tidepool-org/clinic/store/postgres"
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
		shareCodes := make([]sqlcgen.InsertClinicShareCodeParams, 0, len(*clinic.ShareCodes))
		for _, shareCode := range *clinic.ShareCodes {
			shareCodes = append(shareCodes, sqlcgen.InsertClinicShareCodeParams{
				ShareCode: shareCode,
				ClinicID:  id,
			})
		}
		if len(shareCodes) > 0 {
			if err := storepg.ExecBatch(q.InsertClinicShareCode(ctx, shareCodes)); err != nil {
				return err
			}
		}
	}

	if err := q.DeleteClinicAdmins(ctx, id); err != nil {
		return err
	}
	if clinic.Admins != nil {
		admins := make([]sqlcgen.InsertClinicAdminParams, 0, len(*clinic.Admins))
		for _, userId := range *clinic.Admins {
			admins = append(admins, sqlcgen.InsertClinicAdminParams{
				ClinicID: id,
				UserID:   userId,
			})
		}
		if len(admins) > 0 {
			if err := storepg.ExecBatch(q.InsertClinicAdmin(ctx, admins)); err != nil {
				return err
			}
		}
	}

	if err := q.DeleteClinicPhoneNumbers(ctx, id); err != nil {
		return err
	}
	if clinic.PhoneNumbers != nil {
		phoneNumbers := make([]sqlcgen.InsertClinicPhoneNumberParams, 0, len(*clinic.PhoneNumbers))
		for i, phoneNumber := range *clinic.PhoneNumbers {
			phoneNumbers = append(phoneNumbers, sqlcgen.InsertClinicPhoneNumberParams{
				ClinicID: id,
				Ordinal:  int32(i),
				Type:     storepg.TextValue(phoneNumber.Type),
				Number:   phoneNumber.Number,
			})
		}
		if len(phoneNumbers) > 0 {
			if err := storepg.ExecBatch(q.InsertClinicPhoneNumber(ctx, phoneNumbers)); err != nil {
				return err
			}
		}
	}

	if err := q.DeleteClinicMembershipRestrictions(ctx, id); err != nil {
		return err
	}
	restrictions := make([]sqlcgen.InsertClinicMembershipRestrictionParams, 0, len(clinic.MembershipRestrictions))
	for _, restriction := range clinic.MembershipRestrictions {
		restrictions = append(restrictions, sqlcgen.InsertClinicMembershipRestrictionParams{
			ClinicID:    id,
			EmailDomain: restriction.EmailDomain,
			RequiredIdp: pgtype.Text{String: restriction.RequiredIdp, Valid: true},
		})
	}
	if len(restrictions) > 0 {
		if err := storepg.ExecBatch(q.InsertClinicMembershipRestriction(ctx, restrictions)); err != nil {
			return err
		}
	}

	if err := q.DeleteClinicPatientTags(ctx, id); err != nil {
		return err
	}
	tags := make([]sqlcgen.InsertClinicPatientTagParams, 0, len(clinic.PatientTags))
	for _, tag := range clinic.PatientTags {
		if tag.Id == nil {
			return fmt.Errorf("patient tag of clinic %s has no id", id)
		}
		tags = append(tags, sqlcgen.InsertClinicPatientTagParams{
			ID:       tag.Id.Hex(),
			ClinicID: id,
			Name:     tag.Name,
		})
	}
	if len(tags) > 0 {
		if err := storepg.ExecBatch(q.InsertClinicPatientTag(ctx, tags)); err != nil {
			return err
		}
	}

	if err := q.DeleteClinicSites(ctx, id); err != nil {
		return err
	}
	sites := make([]sqlcgen.InsertClinicSiteParams, 0, len(clinic.Sites))
	for _, site := range clinic.Sites {
		sites = append(sites, sqlcgen.InsertClinicSiteParams{
			ID:       site.Id.Hex(),
			ClinicID: id,
			Name:     site.Name,
		})
	}
	if len(sites) > 0 {
		if err := storepg.ExecBatch(q.InsertClinicSite(ctx, sites)); err != nil {
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
		Name:               storepg.TextValue(clinic.Name),
		Address:            storepg.TextValue(clinic.Address),
		City:               storepg.TextValue(clinic.City),
		State:              storepg.TextValue(clinic.State),
		PostalCode:         storepg.TextValue(clinic.PostalCode),
		Country:            storepg.TextValue(clinic.Country),
		ClinicType:         storepg.TextValue(clinic.ClinicType),
		ClinicSize:         storepg.TextValue(clinic.ClinicSize),
		Website:            storepg.TextValue(clinic.Website),
		Timezone:           storepg.TextValue(clinic.Timezone),
		PreferredBgUnits:   storepg.NonEmptyTextValue(clinic.PreferredBgUnits),
		CanonicalShareCode: storepg.TextValue(clinic.CanonicalShareCode),
		Tier:               storepg.NonEmptyTextValue(clinic.Tier),
		IsMigrated:         clinic.IsMigrated,
		CreatedTime:        pgtype.Timestamptz{Time: clinic.CreatedTime.UTC(), Valid: !clinic.CreatedTime.IsZero()},
		UpdatedTime:        pgtype.Timestamptz{Time: clinic.UpdatedTime.UTC(), Valid: !clinic.UpdatedTime.IsZero()},
	}

	if clinic.SuppressedNotifications != nil {
		params.SuppressPatientClinicInvitation = storepg.BoolValue(clinic.SuppressedNotifications.PatientClinicInvitation)
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
		params.EhrProcedureEnableSummaryReports = storepg.TextValue(ehr.ProcedureCodes.EnableSummaryReports)
		params.EhrProcedureDisableSummaryReports = storepg.TextValue(ehr.ProcedureCodes.DisableSummaryReports)
		params.EhrProcedureCreateAccount = storepg.TextValue(ehr.ProcedureCodes.CreateAccount)
		params.EhrProcedureCreateAccountAndEnableReports = storepg.TextValue(ehr.ProcedureCodes.CreateAccountAndEnableReports)
		params.EhrScheduledReportsCadence = pgtype.Text{String: ehr.ScheduledReports.Cadence, Valid: true}
		params.EhrScheduledReportsOnUploadEnabled = pgtype.Bool{Bool: ehr.ScheduledReports.OnUploadEnabled, Valid: true}
		params.EhrScheduledReportsOnUploadNoteEventType = storepg.TextValue(ehr.ScheduledReports.OnUploadNoteEventType)
		params.EhrTagsCodes = ehr.Tags.Codes
		params.EhrTagsSeparator = storepg.TextValue(ehr.Tags.Separator)
		params.EhrFlowsheetsIcode = pgtype.Bool{Bool: ehr.Flowsheets.Icode, Valid: true}
		params.EhrNotesIncludeGmi = pgtype.Bool{Bool: ehr.Notes.IncludeGMI, Valid: true}
	}

	if pcs := clinic.PatientCountSettings; pcs != nil {
		if pcs.HardLimit != nil {
			params.PcsHardLimitPlan = pgtype.Int4{Int32: int32(pcs.HardLimit.Plan), Valid: true}
			params.PcsHardLimitStartDate = storepg.TimestamptzValue(pcs.HardLimit.StartDate)
			params.PcsHardLimitEndDate = storepg.TimestamptzValue(pcs.HardLimit.EndDate)
			params.PcsHardLimitLegacyPatientCount = storepg.IntValue(pcs.HardLimit.PatientCount)
		}
		if pcs.SoftLimit != nil {
			params.PcsSoftLimitPlan = pgtype.Int4{Int32: int32(pcs.SoftLimit.Plan), Valid: true}
			params.PcsSoftLimitStartDate = storepg.TimestamptzValue(pcs.SoftLimit.StartDate)
			params.PcsSoftLimitEndDate = storepg.TimestamptzValue(pcs.SoftLimit.EndDate)
			params.PcsSoftLimitLegacyPatientCount = storepg.IntValue(pcs.SoftLimit.PatientCount)
		}
	}

	if count := clinic.PatientCount; count != nil {
		params.PatientCountTotal = pgtype.Int4{Int32: int32(count.Total), Valid: true}
		params.PatientCountDemo = pgtype.Int4{Int32: int32(count.Demo), Valid: true}
		params.PatientCountPlan = pgtype.Int4{Int32: int32(count.Plan), Valid: true}
		params.PatientCountLegacy = storepg.IntValue(count.PatientCount)
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

// NewBackfiller copies the clinics collection using the same snapshot
// upserts as the dual-write path.
func NewBackfiller(db *mongo.Database, writer *Writer) storepg.Verifier {
	return storepg.NewCollectionSync(db.Collection(clinics.CollectionName), "clinics",
		storepg.UnmarshalAndUpsert(writer.UpsertClinic))
}
