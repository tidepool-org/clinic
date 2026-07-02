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

	"github.com/tidepool-org/clinic/clinicians"
	cliniciansPostgres "github.com/tidepool-org/clinic/clinicians/postgres"
	storepg "github.com/tidepool-org/clinic/store/postgres"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

func newWriter() (*cliniciansPostgres.Writer, *pgx.Conn) {
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

	return cliniciansPostgres.NewWriter(client, zap.NewNop().Sugar()), conn
}

func randomClinician() *clinicians.Clinician {
	id := primitive.NewObjectID()
	clinicId := primitive.NewObjectID()
	userId := "user-" + id.Hex()
	email := "clinician+" + id.Hex() + "@example.com"
	name := "Clinician " + id.Hex()
	return &clinicians.Clinician{
		Id:       &id,
		ClinicId: &clinicId,
		UserId:   &userId,
		Email:    &email,
		Name:     &name,
		Roles:    []string{"CLINIC_ADMIN"},
		RolesUpdates: []clinicians.RolesUpdate{
			{Roles: []string{"CLINIC_MEMBER"}, UpdatedBy: "updater-1"},
			{Roles: []string{"CLINIC_ADMIN"}, UpdatedBy: "updater-2"},
		},
		CreatedTime: time.Now().UTC().Truncate(time.Millisecond),
		UpdatedTime: time.Now().UTC().Truncate(time.Millisecond),
	}
}

var _ = Describe("Clinicians Writer", func() {
	var writer *cliniciansPostgres.Writer
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

	It("upserts clinicians idempotently with the roles audit trail", func() {
		clinician := randomClinician()
		Expect(writer.UpsertClinician(ctx, clinician)).To(Succeed())
		Expect(writer.UpsertClinician(ctx, clinician)).To(Succeed())

		id := clinician.Id.Hex()
		Expect(count("SELECT COUNT(*) FROM clinicians WHERE id = $1 AND clinic_id = $2 AND user_id = $3 AND email = $4 AND 'CLINIC_ADMIN' = ANY(roles)",
			id, clinician.ClinicId.Hex(), *clinician.UserId, *clinician.Email)).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM clinician_roles_updates WHERE clinician_id = $1", id)).To(Equal(2))
	})

	It("stores invites without user ids and clears the invite on association", func() {
		clinician := randomClinician()
		clinician.UserId = nil
		inviteId := "invite-" + clinician.Id.Hex()
		clinician.InviteId = &inviteId
		Expect(writer.UpsertClinician(ctx, clinician)).To(Succeed())

		id := clinician.Id.Hex()
		Expect(count("SELECT COUNT(*) FROM clinicians WHERE id = $1 AND user_id IS NULL AND invite_id = $2", id, inviteId)).To(Equal(1))

		// Associating the invite sets the user id and unsets the invite id
		userId := "user-" + id
		clinician.UserId = &userId
		clinician.InviteId = nil
		Expect(writer.UpsertClinician(ctx, clinician)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM clinicians WHERE id = $1 AND user_id = $2 AND invite_id IS NULL", id, userId)).To(Equal(1))
	})

	It("supports full name searches through the generated search vector", func() {
		clinician := randomClinician()
		name := "Ferdinand Magellan"
		clinician.Name = &name
		Expect(writer.UpsertClinician(ctx, clinician)).To(Succeed())

		Expect(count("SELECT COUNT(*) FROM clinicians WHERE id = $1 AND search_vector @@ to_tsquery('simple', 'magellan')",
			clinician.Id.Hex())).To(Equal(1))
	})

	It("deletes clinicians by membership, clinic and invite", func() {
		member := randomClinician()
		Expect(writer.UpsertClinician(ctx, member)).To(Succeed())
		Expect(writer.DeleteClinician(ctx, member.ClinicId.Hex(), *member.UserId)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM clinicians WHERE id = $1", member.Id.Hex())).To(Equal(0))

		invite := randomClinician()
		invite.UserId = nil
		inviteId := "invite-" + invite.Id.Hex()
		invite.InviteId = &inviteId
		Expect(writer.UpsertClinician(ctx, invite)).To(Succeed())
		Expect(writer.DeleteClinicianInvite(ctx, invite.ClinicId.Hex(), inviteId)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM clinicians WHERE id = $1", invite.Id.Hex())).To(Equal(0))

		all := randomClinician()
		Expect(writer.UpsertClinician(ctx, all)).To(Succeed())
		Expect(writer.DeleteAllClinicians(ctx, all.ClinicId.Hex())).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM clinicians WHERE clinic_id = $1", all.ClinicId.Hex())).To(Equal(0))
	})

	It("rejects clinicians without ids", func() {
		Expect(writer.UpsertClinician(ctx, &clinicians.Clinician{})).ToNot(Succeed())
	})
})

var _ = Describe("Clinicians Backfiller", func() {
	var writer *cliniciansPostgres.Writer
	var conn *pgx.Conn
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
		writer, conn = newWriter()
	})

	It("copies clinicians in resumable batches", func() {
		db := dbTest.GetTestDatabase()
		collection := db.Collection(clinicians.CollectionName)

		// Batch counts below require exclusive ownership of the collection
		_, err := collection.DeleteMany(ctx, bson.M{})
		Expect(err).ToNot(HaveOccurred())

		ids := make([]string, 0, 3)
		for i := 0; i < 3; i++ {
			clinician := randomClinician()
			_, err := collection.InsertOne(ctx, clinician)
			Expect(err).ToNot(HaveOccurred())
			ids = append(ids, clinician.Id.Hex())
		}

		backfiller := cliniciansPostgres.NewBackfiller(db, writer)
		Expect(backfiller.Collection()).To(Equal(clinicians.CollectionName))

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
			Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM clinicians WHERE id = $1", id).Scan(&result)).To(Succeed())
			Expect(result).To(Equal(1))
			Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM clinician_roles_updates WHERE clinician_id = $1", id).Scan(&result)).To(Succeed())
			Expect(result).To(Equal(2))
		}
	})
})
