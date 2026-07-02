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

// Verifies the xealth store mirroring through the HTTP API. A dedicated
// clinic with a unique deployment/source id keeps this isolated from the
// xealth specs, which rely on the shared fixture deployment.
var _ = Describe("Postgres Dual Writes - Xealth", Ordered, func() {
	var deployment string

	BeforeAll(func() {
		admin := newStubUser()
		auth := asUser(admin.UserID)
		clinic := createClinic(auth)
		deployment = fmt.Sprintf("xealth-dw-%s", uniqueId())

		req := prepareRequestWithBody(http.MethodPut,
			fmt.Sprintf("/v1/clinics/%s/settings/ehr", *clinic.Id),
			fixtureWithOverrides("./test/xealth_fixtures/02_enable_xealth.json", map[string]interface{}{
				"sourceId": deployment,
			}))
		asServer(req)
		expectStatus(do(req), http.StatusOK)
	})

	It("mirrors preorder data persisted by subsequent preorder requests", func() {
		// Preorder data is only persisted when the enrollment form is
		// submitted (the "subsequent" preorder event); the initial event
		// just mints the data tracking id.
		req := prepareRequestWithBody(http.MethodPost, "/v1/xealth/preorder",
			fixtureWithOverrides("./test/xealth_fixtures/03_initial_pre_order.json", map[string]interface{}{
				"deployment": deployment,
			}))
		asXealth(req)
		resp := do(req)
		expectStatus(resp, http.StatusOK)

		body := decodeAs[map[string]interface{}](resp)
		trackingId, _ := body["dataTrackingId"].(string)
		Expect(trackingId).ToNot(BeEmpty())

		// The enrollment email must be unique: the fixture's fixed email
		// belongs to the user created by the xealth specs, and a duplicate
		// yields an error form response (still 200) which skips persistence.
		req = prepareRequestWithBody(http.MethodPost, "/v1/xealth/preorder",
			fixtureWithOverrides("./test/xealth_fixtures/04_subsequent_pre_order.json", map[string]interface{}{
				"deployment": deployment,
				"formData": map[string]interface{}{
					"dataTrackingId": trackingId,
					"userInput": map[string]interface{}{
						"patient": map[string]interface{}{
							"email": fmt.Sprintf("xealth-dw+%s@integration.test", uniqueId()),
						},
					},
				},
			}))
		asXealth(req)
		expectStatus(do(req), http.StatusOK)

		Expect(pgCount("xealth_preorders", "data_tracking_id = $1", trackingId)).To(Equal(1))

		// The mirrored row shares the identity of the Mongo document
		var mongoRecord struct {
			Id primitive.ObjectID `bson:"_id"`
		}
		collection := test.GetTestDatabase().Collection("xealth_preorder")
		Expect(collection.FindOne(testCtx(), bson.M{"dataTrackingId": trackingId}).Decode(&mongoRecord)).To(Succeed())
		Expect(pgCount("xealth_preorders", "id = $1", mongoRecord.Id.Hex())).To(Equal(1))
	})
})

// Verifies EHR message mirroring through the redox webhook. The message body
// carries a unique log id so the spec can locate its own documents; message
// processing does not depend on any clinic.
var _ = Describe("Postgres Dual Writes - Redox", Ordered, func() {
	It("mirrors processed EHR messages", func() {
		logId := fmt.Sprintf("dw-log-%s", uniqueId())
		req := prepareRequestWithBody(http.MethodPost, "/v1/redox",
			fixtureWithOverrides("./test/redox_fixtures/04_enable_reports_order.json", map[string]interface{}{
				"Meta": map[string]interface{}{
					"Logs": []interface{}{
						map[string]interface{}{"ID": logId},
					},
				},
			}))
		asRedox(req)
		expectStatus(do(req), http.StatusOK)

		// The Postgres row shares the identity of the Mongo document
		var mongoRecord struct {
			Id primitive.ObjectID `bson:"_id"`
		}
		collection := test.GetTestDatabase().Collection("redox")
		Expect(collection.FindOne(testCtx(), bson.M{"meta.Logs.ID": logId}).Decode(&mongoRecord)).To(Succeed())

		Expect(pgCount("redox_messages",
			"id = $1 AND meta_data_model = 'Order' AND meta_event_type = 'New' AND $2 = ANY(meta_log_ids)",
			mongoRecord.Id.Hex(), logId)).To(Equal(1))
	})
})

// Verifies the clinics aggregate mirroring: the clinics row, all child sets
// and flattened settings converge after every API write.
var _ = Describe("Postgres Dual Writes - Clinics", Ordered, func() {
	var admin shoreline.UserData
	var auth func(*http.Request)
	var clinic client.ClinicV1
	var clinicId string

	BeforeAll(func() {
		admin = newStubUser()
		auth = asUser(admin.UserID)
		clinic = createClinic(auth)
		clinicId = *clinic.Id
	})

	It("mirrors created clinics with share codes and admins", func() {
		Expect(pgCount("clinics", "id = $1 AND name = $2 AND canonical_share_code = $3",
			clinicId, clinic.Name, *clinic.ShareCode)).To(Equal(1))
		Expect(pgCount("clinic_share_codes", "clinic_id = $1 AND share_code = $2",
			clinicId, *clinic.ShareCode)).To(Equal(1))
		Expect(pgCount("clinic_admins", "clinic_id = $1 AND user_id = $2",
			clinicId, admin.UserID)).To(Equal(1))
	})

	It("mirrors EHR settings as flattened columns", func() {
		sourceId := fmt.Sprintf("clinic-dw-%s", uniqueId())
		req := prepareRequestWithBody(http.MethodPut,
			fmt.Sprintf("/v1/clinics/%s/settings/ehr", clinicId),
			fixtureWithOverrides("./test/redox_fixtures/02_enable_redox.json", map[string]interface{}{
				"sourceId": sourceId,
			}))
		asServer(req)
		expectStatus(do(req), http.StatusOK)

		Expect(pgCount("clinics", "id = $1 AND ehr_enabled = true AND ehr_provider = 'redox' AND ehr_source_id = $2",
			clinicId, sourceId)).To(Equal(1))
	})

	It("mirrors patient tags and sites", func() {
		tag := createPatientTag(clinicId, "dwtag-"+uniqueId(), auth)
		site := createSite(clinicId, "dwsite-"+uniqueId(), auth)

		Expect(pgCount("clinic_patient_tags", "id = $1 AND clinic_id = $2 AND name = $3",
			*tag.Id, clinicId, tag.Name)).To(Equal(1))
		Expect(pgCount("clinic_sites", "id = $1 AND clinic_id = $2 AND name = $3",
			site.Id, clinicId, string(site.Name))).To(Equal(1))
	})

	It("mirrors site merges", func() {
		source := createSite(clinicId, "dwsrc-"+uniqueId(), auth)
		target := createSite(clinicId, "dwtgt-"+uniqueId(), auth)

		// The path names the surviving target site; the body names the
		// source site which is removed by the merge
		req := prepareRequestWithBody(http.MethodPost,
			fmt.Sprintf("/v1/clinics/%s/sites/%s/merge", clinicId, target.Id),
			jsonBody(map[string]interface{}{"id": source.Id}))
		asServer(req)
		expectStatus(do(req), http.StatusOK)

		Expect(pgCount("clinic_sites", "id = $1", source.Id)).To(Equal(0))
		Expect(pgCount("clinic_sites", "id = $1", target.Id)).To(Equal(1))
	})

	It("mirrors clinic deletion", func() {
		owner := newStubUser()
		emptyClinic := createClinic(asUser(owner.UserID))

		req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/clinics/%s", *emptyClinic.Id), "")
		asUser(owner.UserID)(req)
		expectStatus(do(req), http.StatusNoContent)

		Expect(pgCount("clinics", "id = $1", *emptyClinic.Id)).To(Equal(0))
		Expect(pgCount("clinic_admins", "clinic_id = $1", *emptyClinic.Id)).To(Equal(0))
	})
})
