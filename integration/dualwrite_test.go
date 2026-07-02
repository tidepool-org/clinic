package integration_test

import (
	"fmt"
	"net/http"
	"time"

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

// Verifies clinician mirroring: memberships, the invite lifecycle and bulk
// email updates.
var _ = Describe("Postgres Dual Writes - Clinicians", Ordered, func() {
	var admin shoreline.UserData
	var auth func(*http.Request)
	var clinicId string

	BeforeAll(func() {
		admin = newStubUser()
		auth = asUser(admin.UserID)
		clinicId = *createClinic(auth).Id
	})

	It("mirrors created clinicians with their roles", func() {
		member := newStubUser()
		clinician := createClinicianDirect(clinicId, member.UserID, "CLINIC_MEMBER")

		Expect(pgCount("clinicians", "clinic_id = $1 AND user_id = $2 AND email = $3 AND roles = '{CLINIC_MEMBER}'",
			clinicId, member.UserID, clinician.Email)).To(Equal(1))
	})

	It("mirrors the invite lifecycle", func() {
		inviteId := fmt.Sprintf("invite-%s", uniqueId())
		email := fmt.Sprintf("invitee+%s@integration.test", uniqueId())
		req := prepareRequestWithBody(http.MethodPost,
			fmt.Sprintf("/v1/clinics/%s/clinicians", clinicId),
			jsonBody(map[string]interface{}{
				"inviteId": inviteId,
				"email":    email,
				"roles":    []string{"CLINIC_MEMBER"},
			}))
		asServer(req)
		expectStatus(do(req), http.StatusOK)

		Expect(pgCount("clinicians", "clinic_id = $1 AND invite_id = $2 AND user_id IS NULL",
			clinicId, inviteId)).To(Equal(1))

		// Associating the invite sets the user id and unsets the invite id
		invitee := newStubUser()
		req = prepareRequestWithBody(http.MethodPatch,
			fmt.Sprintf("/v1/clinics/%s/invites/clinicians/%s/clinician", clinicId, inviteId),
			jsonBody(map[string]interface{}{"userId": invitee.UserID}))
		asServer(req)
		expectStatus(do(req), http.StatusOK)

		Expect(pgCount("clinicians", "clinic_id = $1 AND user_id = $2 AND invite_id IS NULL",
			clinicId, invitee.UserID)).To(Equal(1))
	})

	It("mirrors clinician deletion", func() {
		member := newStubUser()
		createClinicianDirect(clinicId, member.UserID, "CLINIC_MEMBER")
		Expect(pgCount("clinicians", "clinic_id = $1 AND user_id = $2", clinicId, member.UserID)).To(Equal(1))

		req := prepareRequest(http.MethodDelete,
			fmt.Sprintf("/v1/clinics/%s/clinicians/%s", clinicId, member.UserID), "")
		asServer(req)
		expectStatus(do(req), http.StatusOK)

		Expect(pgCount("clinicians", "clinic_id = $1 AND user_id = $2", clinicId, member.UserID)).To(Equal(0))
	})

	It("mirrors bulk email updates", func() {
		member := newStubUser()
		createClinicianDirect(clinicId, member.UserID, "CLINIC_MEMBER")

		updated := fmt.Sprintf("updated+%s@integration.test", uniqueId())
		req := prepareRequestWithBody(http.MethodPost,
			fmt.Sprintf("/v1/users/%s/clinics", member.UserID),
			jsonBody(map[string]interface{}{"email": updated}))
		asServer(req)
		expectStatus(do(req), http.StatusOK)

		Expect(pgCount("clinicians", "clinic_id = $1 AND user_id = $2 AND email = $3",
			clinicId, member.UserID, updated)).To(Equal(1))
	})
})

// Verifies patient mirroring through the HTTP API: row snapshots with
// normalized names and permission flags, bulk tag operations, site renames,
// reviews, cross-clinic data source updates, and cascade deletes.
var _ = Describe("Postgres Dual Writes - Patients", Ordered, func() {
	var admin shoreline.UserData
	var auth func(*http.Request)
	var clinic client.ClinicV1

	BeforeAll(func() {
		admin = newStubUser()
		auth = asUser(admin.UserID)
		clinic = createClinic(auth)
	})

	It("mirrors custodial patients with normalized names and permission flags", func() {
		patient := createCustodialPatient(*clinic.Id, auth, map[string]interface{}{
			"fullName": fmt.Sprintf("Édouard Pátient %s", uniqueId()),
		})

		var mongoRecord struct {
			Id primitive.ObjectID `bson:"_id"`
		}
		collection := test.GetTestDatabase().Collection("patients")
		Expect(collection.FindOne(testCtx(), bson.M{"clinicId": mustObjectId(*clinic.Id), "userId": *patient.Id}).Decode(&mongoRecord)).To(Succeed())

		Expect(pgCount("patients",
			"id = $1 AND clinic_id = $2 AND user_id = $3 AND perm_custodian AND full_name_normalized LIKE 'edouard patient %'",
			mongoRecord.Id.Hex(), *clinic.Id, *patient.Id)).To(Equal(1))
	})

	It("mirrors bulk tag assignment and removal", func() {
		tag := createPatientTag(*clinic.Id, "dw-"+uniqueId(), auth)
		first := createCustodialPatient(*clinic.Id, auth, nil)
		createCustodialPatient(*clinic.Id, auth, nil)

		// A subset of user ids targets only those patients
		req := prepareRequestWithBody(http.MethodPost,
			fmt.Sprintf("/v1/clinics/%s/patients/assign_tag/%s", *clinic.Id, *tag.Id),
			jsonBody([]string{*first.Id}))
		auth(req)
		expectStatus(do(req), http.StatusOK)
		Expect(pgCount("patient_tags", "tag_id = $1", *tag.Id)).To(Equal(1))

		// An empty body targets every patient of the clinic
		req = prepareRequest(http.MethodPost,
			fmt.Sprintf("/v1/clinics/%s/patients/assign_tag/%s", *clinic.Id, *tag.Id), "")
		auth(req)
		expectStatus(do(req), http.StatusOK)
		clinicPatients := pgCount("patients", "clinic_id = $1", *clinic.Id)
		Expect(pgCount("patient_tags", "tag_id = $1", *tag.Id)).To(Equal(clinicPatients))

		req = prepareRequest(http.MethodPost,
			fmt.Sprintf("/v1/clinics/%s/patients/delete_tag/%s", *clinic.Id, *tag.Id), "")
		auth(req)
		expectStatus(do(req), http.StatusOK)
		Expect(pgCount("patient_tags", "tag_id = $1", *tag.Id)).To(Equal(0))
	})

	It("mirrors site assignments and rename propagation", func() {
		site := createSite(*clinic.Id, "dw-site-"+uniqueId(), auth)
		patient := createCustodialPatient(*clinic.Id, auth, map[string]interface{}{
			"sites": []map[string]interface{}{{"id": site.Id, "name": site.Name}},
		})
		Expect(pgCount("patient_sites", "site_id = $1 AND site_name = $2", site.Id, site.Name)).To(Equal(1))

		renamed := "dw-renamed-" + uniqueId()
		req := prepareRequestWithBody(http.MethodPut,
			fmt.Sprintf("/v1/clinics/%s/sites/%s", *clinic.Id, site.Id),
			jsonBody(map[string]interface{}{"id": site.Id, "name": renamed}))
		auth(req)
		expectStatus(do(req), http.StatusOK)
		Expect(pgCount("patient_sites", "site_id = $1 AND site_name = $2", site.Id, renamed)).To(Equal(1))

		_ = patient
	})

	It("mirrors reviews", func() {
		patient := createCustodialPatient(*clinic.Id, auth, nil)
		reviews := addReview(*clinic.Id, *patient.Id, auth)
		Expect(reviews).ToNot(BeEmpty())

		var mongoRecord struct {
			Id primitive.ObjectID `bson:"_id"`
		}
		collection := test.GetTestDatabase().Collection("patients")
		Expect(collection.FindOne(testCtx(), bson.M{"clinicId": mustObjectId(*clinic.Id), "userId": *patient.Id}).Decode(&mongoRecord)).To(Succeed())

		Expect(pgCount("patient_reviews", "patient_id = $1 AND clinician_id = $2",
			mongoRecord.Id.Hex(), admin.UserID)).To(Equal(1))
	})

	It("mirrors data source updates in every clinic", func() {
		other := createClinic(auth)
		patientUser := newStubUser()
		createPatientFromUser(*clinic.Id, patientUser.UserID, asServer, map[string]interface{}{"birthDate": "1991-03-03"})
		createPatientFromUser(*other.Id, patientUser.UserID, asServer, map[string]interface{}{"birthDate": "1991-03-03"})

		req := prepareRequestWithBody(http.MethodPut,
			fmt.Sprintf("/v1/patients/%s/data_sources", patientUser.UserID),
			jsonBody([]map[string]interface{}{{
				"providerName": "dexcom", "state": "connected",
				"dataSourceId": primitive.NewObjectID().Hex(),
			}}))
		asServer(req)
		expectStatus(do(req), http.StatusOK)

		Expect(pgCount(
			"patient_data_sources JOIN patients ON patients.id = patient_data_sources.patient_id",
			"patients.user_id = $1 AND provider_name = 'dexcom' AND state = 'connected'",
			patientUser.UserID)).To(Equal(2))
	})

	It("removes mirrored rows when the last permission is revoked", func() {
		patientUser := newStubUser()
		createPatientFromUser(*clinic.Id, patientUser.UserID, asServer, map[string]interface{}{
			"birthDate":   "1992-04-04",
			"permissions": map[string]interface{}{"view": map[string]interface{}{}},
		})
		Expect(pgCount("patients", "clinic_id = $1 AND user_id = $2", *clinic.Id, patientUser.UserID)).To(Equal(1))

		// Deleting the last permission removes the patient from the clinic
		req := prepareRequest(http.MethodDelete,
			fmt.Sprintf("/v1/clinics/%s/patients/%s/permissions/view", *clinic.Id, patientUser.UserID), "")
		asUser(patientUser.UserID)(req)
		expectStatus(do(req), http.StatusNoContent)

		Expect(pgCount("patients", "clinic_id = $1 AND user_id = $2", *clinic.Id, patientUser.UserID)).To(Equal(0))
	})
})

func mustObjectId(hex string) primitive.ObjectID {
	GinkgoHelper()
	id, err := primitive.ObjectIDFromHex(hex)
	Expect(err).ToNot(HaveOccurred())
	return id
}

// Verifies summary mirroring: summary and period rows land in every clinic
// membership, clear with empty updates, and deletion by summary id removes
// only the matching type.
var _ = Describe("Postgres Dual Writes - Patient Summaries", Ordered, func() {
	var patientUserId string

	summaryBody := func(cgmId, bgmId string, lastData time.Time) map[string]interface{} {
		return mergeSummaries(
			summaryStats("cgm", cgmId,
				map[string]interface{}{"lastData": lastData.Format(time.RFC3339), "hasLastData": true},
				map[string]interface{}{
					"14d": cgmPeriod(map[string]interface{}{"timeInTargetPercent": 0.55, "totalRecords": 777}),
				},
			),
			summaryStats("bgm", bgmId,
				map[string]interface{}{},
				map[string]interface{}{
					"14d": bgmPeriod(map[string]interface{}{"averageGlucoseMmol": 6.9}),
				},
			),
		)
	}

	BeforeAll(func() {
		admin := newStubUser()
		auth := asUser(admin.UserID)
		clinicA := createClinic(auth)
		clinicB := createClinic(auth)

		patientUser := newStubUser()
		patientUserId = patientUser.UserID
		createPatientFromUser(*clinicA.Id, patientUserId, asServer, map[string]interface{}{"birthDate": "1993-05-05"})
		createPatientFromUser(*clinicB.Id, patientUserId, asServer, map[string]interface{}{"birthDate": "1993-05-05"})
	})

	It("mirrors summary and period rows in every clinic", func() {
		cgmId := primitive.NewObjectID().Hex()
		bgmId := primitive.NewObjectID().Hex()
		lastData := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
		seedSummary(patientUserId, summaryBody(cgmId, bgmId, lastData))

		Expect(pgCount(
			"patient_summaries JOIN patients ON patients.id = patient_summaries.patient_id",
			"patients.user_id = $1 AND summary_type = 'cgm' AND summary_id = $2 AND dates_has_last_data",
			patientUserId, cgmId)).To(Equal(2))

		Expect(pgCount(
			"patient_summary_periods JOIN patients ON patients.id = patient_summary_periods.patient_id",
			"patients.user_id = $1 AND summary_type = 'cgm' AND period = '14d'"+
				" AND time_in_target_percent BETWEEN 0.54 AND 0.56 AND total_records = 777",
			patientUserId)).To(Equal(2))

		Expect(pgCount(
			"patient_summary_periods JOIN patients ON patients.id = patient_summary_periods.patient_id",
			"patients.user_id = $1 AND summary_type = 'bgm' AND period = '14d'"+
				" AND average_glucose_mmol BETWEEN 6.89 AND 6.91",
			patientUserId)).To(Equal(2))
	})

	It("clears mirrored rows when the summary is cleared", func() {
		req := prepareRequestWithBody(http.MethodPost,
			fmt.Sprintf("/v1/patients/%s/summary", patientUserId), nil)
		asServer(req)
		expectStatus(do(req), http.StatusOK)

		Expect(pgCount(
			"patient_summaries JOIN patients ON patients.id = patient_summaries.patient_id",
			"patients.user_id = $1", patientUserId)).To(Equal(0))
	})

	It("removes only the matching type when deleting by summary id", func() {
		cgmId := primitive.NewObjectID().Hex()
		bgmId := primitive.NewObjectID().Hex()
		seedSummary(patientUserId, summaryBody(cgmId, bgmId, time.Date(2026, 6, 26, 8, 0, 0, 0, time.UTC)))

		req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/summaries/%s/clinics", cgmId), "")
		asServer(req)
		expectStatus(do(req), http.StatusOK)

		Expect(pgCount("patient_summaries", "summary_id = $1", cgmId)).To(Equal(0))
		Expect(pgCount("patient_summaries", "summary_id = $1", bgmId)).To(Equal(2))
	})
})
