package postgres_test

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/patients"
	patientsPostgres "github.com/tidepool-org/clinic/patients/postgres"
	"github.com/tidepool-org/clinic/sites"
	storepg "github.com/tidepool-org/clinic/store/postgres"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

var _ = DescribeTable("NormalizeFullName",
	func(input, expected string) {
		Expect(patientsPostgres.NormalizeFullName(input)).To(Equal(expected))
	},
	Entry("lowercases", "James Jellyfish", "james jellyfish"),
	Entry("strips diacritics", "Édouard", "edouard"),
	Entry("handles umlauts", "MÜLLER", "muller"),
	Entry("handles combined characters", "Ñoño Çedilla", "nono cedilla"),
	Entry("keeps non-latin characters", "李雷", "李雷"),
	Entry("empty", "", ""),
)

func newTestWriter() (*patientsPostgres.Writer, *pgx.Conn, context.Context) {
	GinkgoHelper()
	ctx := context.Background()
	config := *dbTest.GetTestPostgresConfig()
	config.Enabled = true
	config.RunMigrations = false

	lifecycle := fxtest.NewLifecycle(GinkgoT())
	client, err := storepg.NewClient(&config, zap.NewNop().Sugar(), lifecycle)
	Expect(err).ToNot(HaveOccurred())
	lifecycle.RequireStart()
	DeferCleanup(lifecycle.RequireStop)

	writer := patientsPostgres.NewWriter(client, zap.NewNop().Sugar())

	conn, err := pgx.Connect(ctx, config.ConnectionString())
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() {
		Expect(conn.Close(context.Background())).To(Succeed())
	})
	return writer, conn, ctx
}

func randomPatient(clinicId primitive.ObjectID) *patients.Patient {
	id := primitive.NewObjectID()
	// The ObjectID hex prefix is a timestamp shared by ids generated within
	// the same second; the suffix contains the unique counter
	userId := primitive.NewObjectID().Hex()[14:24]
	fullName := "Édouard " + userId
	birthDate := "1990-01-01"
	email := userId + "@example.com"
	mrn := "mrn-" + userId
	perm := make(patients.Permission)
	return &patients.Patient{
		Id:          &id,
		ClinicId:    &clinicId,
		UserId:      &userId,
		FullName:    &fullName,
		BirthDate:   &birthDate,
		Email:       &email,
		Mrn:         &mrn,
		Permissions: &patients.Permissions{Custodian: &perm, View: &perm},
		CreatedTime: time.Now().UTC().Truncate(time.Millisecond),
		UpdatedTime: time.Now().UTC().Truncate(time.Millisecond),
	}
}

var _ = Describe("Patients Writer", func() {
	var writer *patientsPostgres.Writer
	var conn *pgx.Conn
	var ctx context.Context

	count := func(query string, args ...interface{}) int {
		GinkgoHelper()
		var result int
		Expect(conn.QueryRow(ctx, query, args...).Scan(&result)).To(Succeed())
		return result
	}

	BeforeEach(func() {
		writer, conn, ctx = newTestWriter()
	})

	It("upserts patients idempotently with a normalized name", func() {
		patient := randomPatient(primitive.NewObjectID())
		Expect(writer.UpsertPatient(ctx, patient)).To(Succeed())
		Expect(writer.UpsertPatient(ctx, patient)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM patients WHERE id = $1", patient.Id.Hex())).To(Equal(1))

		var normalized string
		var permCustodian, permView, permUpload bool
		row := conn.QueryRow(ctx,
			"SELECT full_name_normalized, perm_custodian, perm_view, perm_upload FROM patients WHERE id = $1",
			patient.Id.Hex())
		Expect(row.Scan(&normalized, &permCustodian, &permView, &permUpload)).To(Succeed())
		Expect(normalized).To(HavePrefix("edouard "))
		Expect(permCustodian).To(BeTrue())
		Expect(permView).To(BeTrue())
		Expect(permUpload).To(BeFalse())
	})

	It("replaces child sets on upsert", func() {
		patient := randomPatient(primitive.NewObjectID())
		tag1, tag2 := primitive.NewObjectID(), primitive.NewObjectID()
		site1 := sites.Site{Id: primitive.NewObjectID(), Name: "Site One"}
		now := time.Now().UTC().Truncate(time.Millisecond)

		patient.Tags = &[]primitive.ObjectID{tag1, tag2}
		patient.Sites = &[]sites.Site{site1}
		patient.DataSources = &[]patients.DataSource{{ProviderName: "dexcom", State: "pending"}}
		patient.Reviews = []patients.Review{{ClinicianId: "clinician-1", Time: now}}
		patient.ProviderConnectionRequests = patients.ProviderConnectionRequests{
			"dexcom": {{ProviderName: "dexcom", CreatedTime: now}},
		}
		patient.EHRSubscriptions = patients.EHRSubscriptions{
			patients.SubscriptionRedoxSummaryAndReports: {
				Active:   true,
				Provider: "redox",
				MatchedMessages: []patients.MatchedMessage{
					{DocumentId: primitive.NewObjectID(), DataModel: "Order", EventType: "New"},
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
		Expect(writer.UpsertPatient(ctx, patient)).To(Succeed())

		id := patient.Id.Hex()
		Expect(count("SELECT COUNT(*) FROM patient_tags WHERE patient_id = $1", id)).To(Equal(2))
		Expect(count("SELECT COUNT(*) FROM patient_sites WHERE patient_id = $1", id)).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM patient_data_sources WHERE patient_id = $1", id)).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM patient_reviews WHERE patient_id = $1", id)).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM patient_provider_connection_requests WHERE patient_id = $1", id)).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM patient_ehr_subscriptions WHERE patient_id = $1", id)).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM patient_ehr_subscription_matched_messages WHERE patient_id = $1", id)).To(Equal(1))

		// Snapshot replace: dropping children converges to the new state
		patient.Tags = &[]primitive.ObjectID{tag1}
		patient.Sites = nil
		patient.EHRSubscriptions = nil
		Expect(writer.UpsertPatient(ctx, patient)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM patient_tags WHERE patient_id = $1", id)).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM patient_sites WHERE patient_id = $1", id)).To(Equal(0))
		Expect(count("SELECT COUNT(*) FROM patient_ehr_subscriptions WHERE patient_id = $1", id)).To(Equal(0))
		Expect(count("SELECT COUNT(*) FROM patient_ehr_subscription_matched_messages WHERE patient_id = $1", id)).To(Equal(0))
	})

	It("deletes patients with cascading children", func() {
		clinicId := primitive.NewObjectID()
		patient := randomPatient(clinicId)
		patient.Tags = &[]primitive.ObjectID{primitive.NewObjectID()}
		Expect(writer.UpsertPatient(ctx, patient)).To(Succeed())

		Expect(writer.DeletePatient(ctx, clinicId.Hex(), *patient.UserId)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM patients WHERE id = $1", patient.Id.Hex())).To(Equal(0))
		Expect(count("SELECT COUNT(*) FROM patient_tags WHERE patient_id = $1", patient.Id.Hex())).To(Equal(0))
	})

	It("mirrors bulk tag assignment and removal", func() {
		clinicId := primitive.NewObjectID()
		first := randomPatient(clinicId)
		second := randomPatient(clinicId)
		Expect(writer.UpsertPatient(ctx, first)).To(Succeed())
		Expect(writer.UpsertPatient(ctx, second)).To(Succeed())

		tagId := primitive.NewObjectID().Hex()

		// A subset targets only the listed user ids
		Expect(writer.AssignTag(ctx, clinicId.Hex(), tagId, []string{*first.UserId})).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM patient_tags WHERE tag_id = $1", tagId)).To(Equal(1))

		// A nil slice targets every patient of the clinic
		Expect(writer.AssignTag(ctx, clinicId.Hex(), tagId, nil)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM patient_tags WHERE tag_id = $1", tagId)).To(Equal(2))

		Expect(writer.DeleteTag(ctx, clinicId.Hex(), tagId, []string{*second.UserId})).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM patient_tags WHERE tag_id = $1", tagId)).To(Equal(1))

		Expect(writer.DeleteTag(ctx, clinicId.Hex(), tagId, nil)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM patient_tags WHERE tag_id = $1", tagId)).To(Equal(0))
	})

	It("mirrors site renames, merges and tag conversions", func() {
		clinicId := primitive.NewObjectID()
		source := sites.Site{Id: primitive.NewObjectID(), Name: "Source"}
		target := sites.Site{Id: primitive.NewObjectID(), Name: "Target"}

		// first is assigned the source site only; second is assigned both
		first := randomPatient(clinicId)
		first.Sites = &[]sites.Site{source}
		second := randomPatient(clinicId)
		second.Sites = &[]sites.Site{source, target}
		Expect(writer.UpsertPatient(ctx, first)).To(Succeed())
		Expect(writer.UpsertPatient(ctx, second)).To(Succeed())

		Expect(writer.RenameSite(ctx, clinicId.Hex(), source.Id.Hex(), "Renamed")).To(Succeed())
		var name string
		Expect(conn.QueryRow(ctx, "SELECT site_name FROM patient_sites WHERE patient_id = $1 AND site_id = $2",
			first.Id.Hex(), source.Id.Hex()).Scan(&name)).To(Succeed())
		Expect(name).To(Equal("Renamed"))

		Expect(writer.MergeSites(ctx, clinicId.Hex(), source.Id.Hex(), target.Id.Hex(), target.Name)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM patient_sites WHERE site_id = $1", source.Id.Hex())).To(Equal(0))
		// Both patients end with a single membership of the target site
		Expect(count("SELECT COUNT(*) FROM patient_sites WHERE site_id = $1", target.Id.Hex())).To(Equal(2))

		// Converting a tag to a site assigns the site and removes the tag
		tagId := primitive.NewObjectID().Hex()
		converted := sites.Site{Id: primitive.NewObjectID(), Name: "Converted"}
		Expect(writer.AssignTag(ctx, clinicId.Hex(), tagId, []string{*first.UserId})).To(Succeed())
		Expect(writer.ConvertTagToSite(ctx, clinicId.Hex(), tagId, converted.Id.Hex(), converted.Name)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM patient_tags WHERE tag_id = $1", tagId)).To(Equal(0))
		Expect(count("SELECT COUNT(*) FROM patient_sites WHERE site_id = $1", converted.Id.Hex())).To(Equal(1))
	})
})

var _ = Describe("Patients Backfiller", func() {
	var writer *patientsPostgres.Writer
	var conn *pgx.Conn
	var ctx context.Context

	BeforeEach(func() {
		writer, conn, ctx = newTestWriter()
	})

	It("copies patients in resumable batches honoring json struct tags", func() {
		db := dbTest.GetTestDatabase()
		collection := db.Collection(patients.CollectionName)

		// Other specs in this suite may create patients in the shared
		// database; batch counts below require exclusive ownership.
		_, err := collection.DeleteMany(ctx, bson.M{})
		Expect(err).ToNot(HaveOccurred())

		clinicId := primitive.NewObjectID()
		reviewTime := time.Now().UTC().Truncate(time.Millisecond)
		seeded := make([]*patients.Patient, 0, 3)
		for i := 0; i < 3; i++ {
			patient := randomPatient(clinicId)
			patient.Reviews = []patients.Review{{ClinicianId: "clinician-1", Time: reviewTime}}
			_, err := collection.InsertOne(ctx, patient)
			Expect(err).ToNot(HaveOccurred())
			seeded = append(seeded, patient)
		}

		backfiller := patientsPostgres.NewBackfiller(db, writer)
		Expect(backfiller.Collection()).To(Equal(patients.CollectionName))

		last, n, err := backfiller.BackfillBatch(ctx, primitive.NilObjectID, 2)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(2))

		last, n, err = backfiller.BackfillBatch(ctx, last, 2)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(1))

		_, n, err = backfiller.BackfillBatch(ctx, last, 2)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(0))

		for _, patient := range seeded {
			var result int
			Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM patients WHERE id = $1", patient.Id.Hex()).Scan(&result)).To(Succeed())
			Expect(result).To(Equal(1))

			// Reviews only carry json struct tags; the decoder must honor them
			var clinicianId string
			Expect(conn.QueryRow(ctx, "SELECT clinician_id FROM patient_reviews WHERE patient_id = $1", patient.Id.Hex()).Scan(&clinicianId)).To(Succeed())
			Expect(clinicianId).To(Equal("clinician-1"))
		}
	})
})
