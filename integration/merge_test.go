package integration_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	integrationTest "github.com/tidepool-org/clinic/integration/test"
	"github.com/tidepool-org/clinic/pointer"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

var _ = Describe("Merge Test", Ordered, func() {
	// The test user is an admin of target and source, a member of sourceAsMember and absent from sourceWithoutMembership
	var target, source, sourceAsMember, sourceWithoutMembership, lone primitive.ObjectID

	newClinic := func(role string) primitive.ObjectID {
		db := dbTest.GetTestDatabase()
		id := primitive.NewObjectID()
		shareCode := id.Hex()
		_, err := db.Collection("clinics").InsertOne(testCtx(), clinics.Clinic{
			Id:                 &id,
			Name:               pointer.FromAny("Merge " + id.Hex()),
			ShareCodes:         &[]string{shareCode},
			CanonicalShareCode: &shareCode,
		})
		Expect(err).ToNot(HaveOccurred())
		if role != "" {
			_, err = db.Collection("clinicians").InsertOne(testCtx(), clinicians.Clinician{
				ClinicId: &id,
				UserId:   pointer.FromAny(integrationTest.TestUserId),
				Name:     pointer.FromAny("Integration Admin"),
				Email:    pointer.FromAny("admin@example.test"),
				Roles:    []string{role},
			})
			Expect(err).ToNot(HaveOccurred())
		}
		return id
	}

	post := func(endpoint, body string, as func(*http.Request)) int {
		rec := httptest.NewRecorder()
		req := prepareRequestWithBody(http.MethodPost, endpoint, strings.NewReader(body))
		as(req)
		server.ServeHTTP(rec, req)
		return rec.Result().StatusCode
	}
	report := func(clinicId primitive.ObjectID, body string) int {
		return post(fmt.Sprintf("/v1/clinics/%s/reports/merge", clinicId.Hex()), body, asClinician)
	}
	mergeInto := func(clinicId primitive.ObjectID, body string, as func(*http.Request)) int {
		return post(fmt.Sprintf("/v1/clinics/%s/merge", clinicId.Hex()), body, as)
	}
	sourceBody := func(sourceId primitive.ObjectID, extra string) string {
		return fmt.Sprintf(`{"sourceId":%q%s}`, sourceId.Hex(), extra)
	}
	clinicExists := func(id primitive.ObjectID) bool {
		count, err := dbTest.GetTestDatabase().Collection("clinics").CountDocuments(testCtx(), bson.M{"_id": id})
		Expect(err).ToNot(HaveOccurred())
		return count == 1
	}
	persistedClinicPlan := func(targetId primitive.ObjectID) bson.M {
		var persisted struct {
			Plan bson.M `bson:"plan"`
		}
		filter := bson.M{"type": "clinic", "plan.target._id": targetId}
		Expect(dbTest.GetTestDatabase().Collection("merge_plans").FindOne(testCtx(), filter).Decode(&persisted)).To(Succeed())
		return persisted.Plan
	}

	BeforeAll(func() {
		target = newClinic(clinicians.RoleClinicAdmin)
		source = newClinic(clinicians.RoleClinicAdmin)
		sourceAsMember = newClinic(clinicians.RoleClinicMember)
		sourceWithoutMembership = newClinic("")
		lone = newClinic(clinicians.RoleClinicAdmin)
	})

	It("allows an admin of both clinics to generate the merge report", func() {
		Expect(report(target, sourceBody(source, ""))).To(Equal(http.StatusOK))
	})

	It("denies the merge report to a target admin who is only a member of the source", func() {
		Expect(report(target, sourceBody(sourceAsMember, ""))).To(Equal(http.StatusForbidden))
	})

	It("denies the merge report to a target admin who is not a member of the source", func() {
		Expect(report(target, sourceBody(sourceWithoutMembership, ""))).To(Equal(http.StatusForbidden))
	})

	It("denies the merge to a target admin who is only a member of the source", func() {
		Expect(mergeInto(target, sourceBody(sourceAsMember, `,"confirmed":true`), asClinician)).To(Equal(http.StatusForbidden))
		Expect(clinicExists(sourceAsMember)).To(BeTrue())
	})

	It("denies the merge to a target admin who is not a member of the source", func() {
		Expect(mergeInto(target, sourceBody(sourceWithoutMembership, `,"confirmed":true`), asClinician)).To(Equal(http.StatusForbidden))
		Expect(clinicExists(sourceWithoutMembership)).To(BeTrue())
	})

	It("allows the admin of a single clinic to generate its duplicates report", func() {
		Expect(report(lone, `{}`)).To(Equal(http.StatusOK))
	})

	It("rejects a clinician merge without confirmation and changes nothing", func() {
		Expect(mergeInto(target, sourceBody(source, ""), asClinician)).To(Equal(http.StatusBadRequest))
		Expect(mergeInto(target, sourceBody(source, `,"confirmed":false`), asClinician)).To(Equal(http.StatusBadRequest))
		Expect(clinicExists(source)).To(BeTrue())
		count, err := dbTest.GetTestDatabase().Collection("merge_plans").CountDocuments(testCtx(), bson.M{"plan.target._id": target})
		Expect(err).ToNot(HaveOccurred())
		Expect(count).To(BeZero())
	})

	It("merges for an admin of both clinics who confirmed and records the consent", func() {
		Expect(mergeInto(target, sourceBody(source, `,"confirmed":true`), asClinician)).To(Equal(http.StatusOK))
		Expect(clinicExists(source)).To(BeFalse())

		consent, ok := persistedClinicPlan(target)["consent"].(bson.M)
		Expect(ok).To(BeTrue())
		Expect(consent["confirmingUserId"]).To(Equal(integrationTest.TestUserId))
		Expect(consent["confirmed"]).To(BeTrue())
		Expect(consent["sourceDeleted"]).To(BeTrue())
	})

	It("still merges for backend services without confirmation and records no consent", func() {
		serverTarget := newClinic(clinicians.RoleClinicAdmin)
		serverSource := newClinic(clinicians.RoleClinicAdmin)
		Expect(mergeInto(serverTarget, sourceBody(serverSource, ""), asServer)).To(Equal(http.StatusOK))
		Expect(clinicExists(serverSource)).To(BeFalse())
		Expect(persistedClinicPlan(serverTarget)).ToNot(HaveKey("consent"))
	})
})
