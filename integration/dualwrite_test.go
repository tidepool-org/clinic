package integration_test

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Verifies that Mongo writes are mirrored to Postgres: deletion audit
// records, clinic migrations, and merge plans. Mirror writes are flushed
// synchronously (post-commit for transactional paths), so rows must exist as
// soon as the API call returns.
var _ = Describe("Postgres Dual Writes", Ordered, func() {
	var admin shoreline.UserData
	var auth func(*http.Request)
	var clinic client.ClinicV1

	BeforeAll(func() {
		admin = newStubUser()
		auth = asUser(admin.UserID)
		clinic = createClinic(auth)
	})

	It("mirrors patient deletion records", func() {
		patient := createCustodialPatient(*clinic.Id, auth, nil)

		req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, *patient.Id), "")
		auth(req)
		expectStatus(do(req), http.StatusNoContent)

		// The Postgres record shares the identity of the Mongo record
		var mongoRecord struct {
			Id primitive.ObjectID `bson:"_id"`
		}
		collection := test.GetTestDatabase().Collection("patient_deletions")
		Expect(collection.FindOne(testCtx(), bson.M{"patient.userId": *patient.Id}).Decode(&mongoRecord)).To(Succeed())

		Expect(pgCount("patient_deletions", "id = $1 AND clinic_id = $2 AND user_id = $3 AND deleted_by_user_id = $4",
			mongoRecord.Id.Hex(), *clinic.Id, *patient.Id, admin.UserID)).To(Equal(1))
	})

	It("mirrors clinician deletion records", func() {
		member := newStubUser()
		createClinicianDirect(*clinic.Id, member.UserID, "CLINIC_MEMBER")

		req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", *clinic.Id, member.UserID), "")
		asServer(req)
		expectStatus(do(req), http.StatusOK)

		var mongoRecord struct {
			Id primitive.ObjectID `bson:"_id"`
		}
		collection := test.GetTestDatabase().Collection("clinician_deletions")
		Expect(collection.FindOne(testCtx(), bson.M{"clinician.userId": member.UserID}).Decode(&mongoRecord)).To(Succeed())

		Expect(pgCount("clinician_deletions", "id = $1 AND clinic_id = $2 AND user_id = $3",
			mongoRecord.Id.Hex(), *clinic.Id, member.UserID)).To(Equal(1))
	})

	It("mirrors clinic deletion records", func() {
		owner := newStubUser()
		emptyClinic := createClinic(asUser(owner.UserID))

		req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/clinics/%s", *emptyClinic.Id), "")
		asUser(owner.UserID)(req)
		expectStatus(do(req), http.StatusNoContent)

		clinicObjId, err := primitive.ObjectIDFromHex(*emptyClinic.Id)
		Expect(err).ToNot(HaveOccurred())
		var mongoRecord struct {
			Id primitive.ObjectID `bson:"_id"`
		}
		collection := test.GetTestDatabase().Collection("clinic_deletions")
		Expect(collection.FindOne(testCtx(), bson.M{"clinic._id": clinicObjId}).Decode(&mongoRecord)).To(Succeed())

		Expect(pgCount("clinic_deletions", "id = $1 AND clinic_id = $2",
			mongoRecord.Id.Hex(), *emptyClinic.Id)).To(Equal(1))
	})

	It("mirrors clinic migrations", func() {
		// The migration flow requires a legacy clinic user (shoreline role
		// "clinic") that administers a single clinic with a complete profile
		email := fmt.Sprintf("legacy+%s@integration.test", uniqueId())
		legacy := shoreline.UserData{
			UserID:        stubUsers.NextUserId(),
			Username:      email,
			Emails:        []string{email},
			Roles:         []string{"clinic"},
			EmailVerified: true,
		}
		stubUsers.AddUser(legacy)

		req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinicians/%s/migrate", legacy.UserID), "")
		asServer(req)
		resp := do(req)
		expectStatus(resp, http.StatusOK)
		legacyClinic := decodeAs[client.ClinicV1](resp)

		req = prepareRequest(http.MethodPut, fmt.Sprintf("/v1/clinics/%s", *legacyClinic.Id), "./test/migrate_fixtures/02_update_clinic_complete.json")
		asServer(req)
		expectStatus(do(req), http.StatusOK)

		// Triggering the initial migration requires clinician write access,
		// so it is invoked as the legacy admin rather than a backend service
		req = prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/migrate", *legacyClinic.Id), "")
		asUser(legacy.UserID)(req)
		expectStatus(do(req), http.StatusOK)

		Expect(pgCount("migrations", "user_id = $1 AND clinic_id = $2 AND status = 'PENDING'",
			legacy.UserID, *legacyClinic.Id)).To(Equal(1))
	})

	It("mirrors merge plans", func() {
		owner := newStubUser()
		source := createClinic(asUser(owner.UserID))
		target := createClinic(asUser(owner.UserID))
		createCustodialPatient(*source.Id, asUser(owner.UserID), nil)
		createCustodialPatient(*target.Id, asUser(owner.UserID), nil)

		req := prepareRequestWithBody(http.MethodPost,
			fmt.Sprintf("/v1/clinics/%s/merge", *target.Id),
			jsonBody(map[string]interface{}{"sourceId": *source.Id}))
		asServer(req)
		expectStatus(do(req), http.StatusOK)

		// Every persisted plan document must have a mirrored row with the
		// same identity
		collection := test.GetTestDatabase().Collection("merge_plans")
		cursor, err := collection.Find(testCtx(), bson.M{})
		Expect(err).ToNot(HaveOccurred())
		var plans []struct {
			Id     primitive.ObjectID `bson:"_id"`
			PlanId primitive.ObjectID `bson:"planId"`
			Type   string             `bson:"type"`
		}
		Expect(cursor.All(testCtx(), &plans)).To(Succeed())
		Expect(plans).ToNot(BeEmpty())

		for _, plan := range plans {
			Expect(pgCount("merge_plans", "id = $1 AND plan_id = $2 AND type = $3",
				plan.Id.Hex(), plan.PlanId.Hex(), plan.Type)).To(Equal(1), "expected mirrored merge plan %s", plan.Id.Hex())
		}
	})
})
