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

	"github.com/tidepool-org/clinic/clinics"
	clinicsPostgres "github.com/tidepool-org/clinic/clinics/postgres"
	"github.com/tidepool-org/clinic/sites"
	storepg "github.com/tidepool-org/clinic/store/postgres"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

func newWriter() (*clinicsPostgres.Writer, *pgx.Conn) {
	GinkgoHelper()
	config := *dbTest.GetTestPostgresConfig()
	config.Enabled = true
	config.RunMigrations = false

	lifecycle := fxtest.NewLifecycle(GinkgoT())
	client, err := storepg.NewClient(&config, zap.NewNop().Sugar(), lifecycle)
	Expect(err).ToNot(HaveOccurred())
	lifecycle.RequireStart()
	DeferCleanup(lifecycle.RequireStop)

	conn, err := pgx.Connect(context.Background(), config.ConnectionString())
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() {
		Expect(conn.Close(context.Background())).To(Succeed())
	})

	return clinicsPostgres.NewWriter(client, zap.NewNop().Sugar()), conn
}

func randomClinic() *clinics.Clinic {
	id := primitive.NewObjectID()
	name := "Clinic " + id.Hex()
	address := "1 Main St"
	country := "US"
	shareCode := "SHARE-" + id.Hex()
	admins := []string{"admin-1", "admin-2"}
	phoneType := "main"
	return &clinics.Clinic{
		Id:                 &id,
		Name:               &name,
		Address:            &address,
		Country:            &country,
		CanonicalShareCode: &shareCode,
		ShareCodes:         &[]string{shareCode},
		Admins:             &admins,
		PhoneNumbers:       &[]clinics.PhoneNumber{{Type: &phoneType, Number: "555"}},
		CreatedTime:        time.Now().UTC().Truncate(time.Millisecond),
		UpdatedTime:        time.Now().UTC().Truncate(time.Millisecond),
		Tier:               "tier0100",
		PreferredBgUnits:   "mg/dL",
		PatientTags: []clinics.PatientTag{
			{Id: newObjectId(), Name: "tag-a"},
			{Id: newObjectId(), Name: "tag-b"},
		},
		Sites: []sites.Site{
			{Id: primitive.NewObjectID(), Name: "site-a"},
		},
		MembershipRestrictions: []clinics.MembershipRestrictions{
			{EmailDomain: "example.com", RequiredIdp: "idp"},
		},
		EHRSettings: &clinics.EHRSettings{
			Enabled:  true,
			Provider: "redox",
			SourceId: "source-" + id.Hex(),
			ScheduledReports: clinics.ScheduledReports{
				Cadence:         "14d",
				OnUploadEnabled: true,
			},
			Tags: clinics.TagsSettings{Codes: []string{"a", "b"}},
		},
		PatientCountSettings: &clinics.PatientCountSettings{
			HardLimit: &clinics.PatientCountLimit{Plan: 250},
		},
		PatientCount: &clinics.PatientCount{
			Total: 5, Demo: 1, Plan: 4,
			Providers: map[string]clinics.PatientProviderCount{
				"dexcom": {Total: 2, States: map[string]int{"connected": 2}},
			},
		},
	}
}

func newObjectId() *primitive.ObjectID {
	id := primitive.NewObjectID()
	return &id
}

var _ = Describe("Clinics Writer", func() {
	var writer *clinicsPostgres.Writer
	var conn *pgx.Conn
	var ctx context.Context

	count := func(query string, args ...interface{}) int {
		GinkgoHelper()
		var result int
		Expect(conn.QueryRow(ctx, query, args...).Scan(&result)).To(Succeed())
		return result
	}

	BeforeEach(func() {
		ctx = context.Background()
		writer, conn = newWriter()
	})

	It("upserts clinics idempotently with all child sets", func() {
		clinic := randomClinic()
		Expect(writer.UpsertClinic(ctx, clinic)).To(Succeed())
		Expect(writer.UpsertClinic(ctx, clinic)).To(Succeed())

		id := clinic.Id.Hex()
		Expect(count("SELECT COUNT(*) FROM clinics WHERE id = $1", id)).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM clinic_share_codes WHERE clinic_id = $1", id)).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM clinic_admins WHERE clinic_id = $1", id)).To(Equal(2))
		Expect(count("SELECT COUNT(*) FROM clinic_phone_numbers WHERE clinic_id = $1", id)).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM clinic_membership_restrictions WHERE clinic_id = $1", id)).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM clinic_patient_tags WHERE clinic_id = $1", id)).To(Equal(2))
		Expect(count("SELECT COUNT(*) FROM clinic_sites WHERE clinic_id = $1", id)).To(Equal(1))

		var name, country, ehrProvider, ehrSourceId string
		var mrnRequired *bool
		var pcsHardLimitPlan int
		row := conn.QueryRow(ctx,
			"SELECT name, country, ehr_provider, ehr_source_id, mrn_required, pcs_hard_limit_plan FROM clinics WHERE id = $1", id)
		Expect(row.Scan(&name, &country, &ehrProvider, &ehrSourceId, &mrnRequired, &pcsHardLimitPlan)).To(Succeed())
		Expect(name).To(Equal(*clinic.Name))
		Expect(country).To(Equal("US"))
		Expect(ehrProvider).To(Equal("redox"))
		Expect(ehrSourceId).To(Equal(clinic.EHRSettings.SourceId))
		Expect(mrnRequired).To(BeNil())
		Expect(pcsHardLimitPlan).To(Equal(250))
	})

	It("replaces child sets on subsequent upserts", func() {
		clinic := randomClinic()
		Expect(writer.UpsertClinic(ctx, clinic)).To(Succeed())

		// Remove a tag, rename the site, drop an admin
		clinic.PatientTags = clinic.PatientTags[:1]
		clinic.Sites[0].Name = "site-renamed"
		admins := (*clinic.Admins)[:1]
		clinic.Admins = &admins
		Expect(writer.UpsertClinic(ctx, clinic)).To(Succeed())

		id := clinic.Id.Hex()
		Expect(count("SELECT COUNT(*) FROM clinic_patient_tags WHERE clinic_id = $1", id)).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM clinic_admins WHERE clinic_id = $1", id)).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM clinic_sites WHERE clinic_id = $1 AND name = 'site-renamed'", id)).To(Equal(1))
	})

	It("deletes clinics and cascades to children", func() {
		clinic := randomClinic()
		Expect(writer.UpsertClinic(ctx, clinic)).To(Succeed())

		id := clinic.Id.Hex()
		Expect(writer.DeleteClinic(ctx, id)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM clinics WHERE id = $1", id)).To(Equal(0))
		Expect(count("SELECT COUNT(*) FROM clinic_share_codes WHERE clinic_id = $1", id)).To(Equal(0))
		Expect(count("SELECT COUNT(*) FROM clinic_admins WHERE clinic_id = $1", id)).To(Equal(0))
	})

	It("rejects clinics without ids", func() {
		Expect(writer.UpsertClinic(ctx, &clinics.Clinic{})).ToNot(Succeed())
	})
})

var _ = Describe("Clinics Backfiller", func() {
	var writer *clinicsPostgres.Writer
	var conn *pgx.Conn
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
		writer, conn = newWriter()
	})

	It("copies clinics in resumable batches", func() {
		db := dbTest.GetTestDatabase()
		collection := db.Collection(clinics.CollectionName)

		// Batch counts below require exclusive ownership of the collection
		_, err := collection.DeleteMany(ctx, bson.M{})
		Expect(err).ToNot(HaveOccurred())

		ids := make([]string, 0, 3)
		for i := 0; i < 3; i++ {
			clinic := randomClinic()
			_, err := collection.InsertOne(ctx, clinic)
			Expect(err).ToNot(HaveOccurred())
			ids = append(ids, clinic.Id.Hex())
		}

		backfiller := clinicsPostgres.NewBackfiller(db, writer)
		Expect(backfiller.Collection()).To(Equal(clinics.CollectionName))

		last, n, err := backfiller.BackfillBatch(ctx, primitive.NilObjectID, 2)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(2))

		last, n, err = backfiller.BackfillBatch(ctx, last, 2)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(1))

		_, n, err = backfiller.BackfillBatch(ctx, last, 2)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(0))

		for _, id := range ids {
			var result int
			Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM clinics WHERE id = $1", id).Scan(&result)).To(Succeed())
			Expect(result).To(Equal(1))
			Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM clinic_admins WHERE clinic_id = $1", id).Scan(&result)).To(Succeed())
			Expect(result).To(Equal(2))
		}
	})
})
