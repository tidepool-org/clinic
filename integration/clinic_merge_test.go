package integration_test

import (
	"fmt"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/clinics/merge"
	"github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Pins the clinic merge: report categorization (duplicate / likely duplicate /
// MRN-only matches), clinician and tag/site plans, MRN uniqueness blockers,
// and the effects of executing a merge (patients and clinicians moved and
// de-duplicated, same-name tags unified, colliding site names renamed).
// Both endpoints are backend-service-only. The JSON report is the domain
// merge.ClinicMergePlan, serialized with Go field names.
var _ = Describe("Clinic Merge", Ordered, func() {
	var admin func(*http.Request)

	generateReport := func(targetId, sourceId string) merge.ClinicMergePlan {
		GinkgoHelper()
		req := prepareRequestWithBody(http.MethodPost,
			fmt.Sprintf("/v1/clinics/%s/reports/merge", targetId),
			jsonBody(map[string]interface{}{"sourceId": sourceId}))
		req.Header.Set("Accept", "application/json")
		asServer(req)
		resp := do(req)
		expectStatus(resp, http.StatusOK)
		return decodeAs[merge.ClinicMergePlan](resp)
	}

	executeMerge := func(targetId, sourceId string, expectedStatus int) {
		GinkgoHelper()
		req := prepareRequestWithBody(http.MethodPost,
			fmt.Sprintf("/v1/clinics/%s/merge", targetId),
			jsonBody(map[string]interface{}{"sourceId": sourceId}))
		asServer(req)
		resp := do(req)
		expectStatus(resp, expectedStatus)
	}

	patientPlanFor := func(plan merge.ClinicMergePlan, userId string) merge.PatientPlan {
		GinkgoHelper()
		for _, p := range plan.PatientPlans {
			if p.SourcePatient != nil && p.SourcePatient.UserId != nil && *p.SourcePatient.UserId == userId {
				return p
			}
		}
		Fail(fmt.Sprintf("no patient plan found for source patient %s", userId))
		return merge.PatientPlan{}
	}

	BeforeAll(func() {
		adminUser := newStubUser()
		admin = asUser(adminUser.UserID)
	})

	Describe("authorization", func() {
		It("rejects report generation and merges from clinic admins", func() {
			source := createClinic(admin)
			target := createClinic(admin)

			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/reports/merge", *target.Id),
				jsonBody(map[string]interface{}{"sourceId": *source.Id}))
			req.Header.Set("Accept", "application/json")
			admin(req)
			expectStatus(do(req), http.StatusForbidden)

			req = prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/merge", *target.Id),
				jsonBody(map[string]interface{}{"sourceId": *source.Id}))
			admin(req)
			expectStatus(do(req), http.StatusForbidden)
		})
	})

	Describe("merge report and execution", func() {
		var fam string
		var source, target client.ClinicV1
		var overlapClinician, sourceClinician client.ClinicianV1
		var duplicateUserId string
		var likelySource, likelyTarget, mrnSource, mrnTarget, uniqueSource, uniqueTarget client.PatientV1
		var sharedSourceTag, sharedTargetTag, sourceOnlyTag client.PatientTagV1
		var sourceSite, targetSite client.SiteV1
		var report merge.ClinicMergePlan

		BeforeAll(func() {
			fam = uniqueId()
			source = createClinic(admin)
			target = createClinic(admin)

			overlap := newStubUser()
			overlapClinician = createClinicianDirect(*source.Id, overlap.UserID, "CLINIC_MEMBER")
			createClinicianDirect(*target.Id, overlap.UserID, "CLINIC_MEMBER")

			srcOnly := newStubUser()
			sourceClinician = createClinicianDirect(*source.Id, srcOnly.UserID, "CLINIC_MEMBER")

			// Same user account in both clinics.
			dup := newStubUser()
			duplicateUserId = dup.UserID
			createPatientFromUser(*source.Id, duplicateUserId, asServer, map[string]interface{}{"birthDate": "1975-03-03"})
			createPatientFromUser(*target.Id, duplicateUserId, asServer, map[string]interface{}{"birthDate": "1975-03-03"})

			// Same full name and birth date, different accounts.
			likelyName := "Likely Dup " + fam
			likelySource = createCustodialPatient(*source.Id, admin, map[string]interface{}{
				"fullName": likelyName, "birthDate": "1980-01-01",
			})
			likelyTarget = createCustodialPatient(*target.Id, admin, map[string]interface{}{
				"fullName": likelyName, "birthDate": "1980-01-01",
			})

			// Same MRN, different names and birth dates.
			mrn := "MRN" + fam
			mrnSource = createCustodialPatient(*source.Id, admin, map[string]interface{}{
				"fullName": "Mrn Source " + fam, "birthDate": "1970-02-02", "mrn": mrn,
			})
			mrnTarget = createCustodialPatient(*target.Id, admin, map[string]interface{}{
				"fullName": "Mrn Target " + fam, "birthDate": "1971-03-03", "mrn": mrn,
			})

			sharedSourceTag = createPatientTag(*source.Id, "shared-"+fam, admin)
			sharedTargetTag = createPatientTag(*target.Id, "shared-"+fam, admin)
			sourceOnlyTag = createPatientTag(*source.Id, "srconly-"+fam, admin)

			sourceSite = createSite(*source.Id, "site-"+fam, admin)
			targetSite = createSite(*target.Id, "site-"+fam, admin)

			uniqueSource = createCustodialPatient(*source.Id, admin, map[string]interface{}{
				"fullName": "Unique Source " + fam, "birthDate": "1965-05-05",
				"tags":  []string{*sharedSourceTag.Id, *sourceOnlyTag.Id},
				"sites": []map[string]interface{}{{"id": sourceSite.Id, "name": sourceSite.Name}},
			})
			uniqueTarget = createCustodialPatient(*target.Id, admin, map[string]interface{}{
				"fullName": "Unique Target " + fam, "birthDate": "1966-06-06",
			})

			report = generateReport(*target.Id, *source.Id)
		})

		It("allows the merge", func() {
			Expect(report.PreventsMerge()).To(BeFalse())
		})

		It("categorizes duplicate accounts and marks them for merging", func() {
			plan := patientPlanFor(report, duplicateUserId)
			Expect(plan.PatientAction).To(Equal(merge.PatientActionMerge))
			Expect(plan.Conflicts).To(HaveKey(merge.PatientConflictCategoryDuplicateAccounts))
			conflicts := plan.Conflicts[merge.PatientConflictCategoryDuplicateAccounts]
			Expect(conflicts).To(HaveLen(1))
			Expect(conflicts[0].Patient.UserId).To(HaveValue(Equal(duplicateUserId)))
		})

		It("categorizes likely duplicate accounts", func() {
			plan := patientPlanFor(report, *likelySource.Id)
			Expect(plan.PatientAction).To(Equal(merge.PatientActionMove))
			Expect(plan.Conflicts).To(HaveKey(merge.PatientConflictCategoryLikelyDuplicateAccounts))
			conflicts := plan.Conflicts[merge.PatientConflictCategoryLikelyDuplicateAccounts]
			Expect(conflicts).To(HaveLen(1))
			Expect(conflicts[0].Patient.UserId).To(HaveValue(Equal(*likelyTarget.Id)))
		})

		It("categorizes MRN-only matches", func() {
			plan := patientPlanFor(report, *mrnSource.Id)
			Expect(plan.Conflicts).To(HaveKey(merge.PatientConflictCategoryMRNOnlyMatch))
			conflicts := plan.Conflicts[merge.PatientConflictCategoryMRNOnlyMatch]
			Expect(conflicts).To(HaveLen(1))
			Expect(conflicts[0].Patient.UserId).To(HaveValue(Equal(*mrnTarget.Id)))
		})

		It("plans clinician moves and merges", func() {
			// Overlapping clinicians produce two plans: MERGE for the source
			// membership and MERGE_INTO for the target membership.
			actionsByUserId := map[string][]string{}
			for _, p := range report.ClinicianPlans {
				if p.Clinician.UserId != nil {
					actionsByUserId[*p.Clinician.UserId] = append(actionsByUserId[*p.Clinician.UserId], p.ClinicianAction)
				}
			}
			Expect(actionsByUserId[*sourceClinician.Id]).To(ConsistOf(merge.ClinicianActionMove))
			Expect(actionsByUserId[*overlapClinician.Id]).To(ConsistOf(merge.ClinicianActionMerge, merge.ClinicianActionMergeInto))
		})

		It("plans tag unification and creation", func() {
			plansByName := map[string]merge.TagPlan{}
			for _, p := range report.TagsPlans {
				plansByName[p.Name] = p
			}
			Expect(plansByName).To(HaveKey("shared-" + fam))
			Expect(plansByName["shared-"+fam].Merge).To(BeTrue())
			Expect(plansByName).To(HaveKey("srconly-" + fam))
			Expect(plansByName["srconly-"+fam].TagAction).To(Equal(merge.TagActionCreate))
		})

		It("plans renames for colliding site names", func() {
			var sourceSitePlan *merge.SitePlan
			for i, p := range report.SitesPlans {
				if p.Site.Id.Hex() == sourceSite.Id {
					sourceSitePlan = &report.SitesPlans[i]
				}
			}
			Expect(sourceSitePlan).ToNot(BeNil())
			Expect(sourceSitePlan.Action).To(Equal(merge.SiteActionRename))
			Expect(sourceSitePlan.ExpectedRename).To(Equal(fmt.Sprintf("site-%s (2)", fam)))
		})

		It("generates an XLSX report by default", func() {
			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/reports/merge", *target.Id),
				jsonBody(map[string]interface{}{"sourceId": *source.Id}))
			asServer(req)
			resp := do(req)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/vnd.ms-excel"))
			body, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(body).ToNot(BeEmpty())
		})

		When("the merge is executed", func() {
			BeforeAll(func() {
				executeMerge(*target.Id, *source.Id, http.StatusOK)
			})

			It("moves source patients into the target clinic", func() {
				for _, patientId := range []string{*likelySource.Id, *mrnSource.Id, *uniqueSource.Id} {
					patient := getPatient(*target.Id, patientId)
					Expect(patient.Id).To(HaveValue(Equal(patientId)))
				}
				Expect(getPatient(*target.Id, *uniqueTarget.Id).Id).To(HaveValue(Equal(*uniqueTarget.Id)))
			})

			It("does not duplicate patients present in both clinics", func() {
				response := listPatients(*target.Id, nil, asServer)
				count := 0
				for _, p := range *response.Data {
					if p.Id != nil && *p.Id == duplicateUserId {
						count++
					}
				}
				Expect(count).To(Equal(1))
			})

			It("moves source clinicians and keeps overlapping ones", func() {
				Expect(getClinician(*target.Id, *sourceClinician.Id).Id).To(HaveValue(Equal(*sourceClinician.Id)))
				Expect(getClinician(*target.Id, *overlapClinician.Id).Id).To(HaveValue(Equal(*overlapClinician.Id)))
			})

			It("unifies same-name tags and recreates source-only tags", func() {
				req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s", *target.Id), "")
				asServer(req)
				resp := do(req)
				expectStatus(resp, http.StatusOK)
				clinic := decodeAs[client.ClinicV1](resp)

				Expect(clinic.PatientTags).ToNot(BeNil())
				tagIdsByName := map[string]string{}
				for _, tag := range *clinic.PatientTags {
					tagIdsByName[tag.Name] = *tag.Id
				}
				Expect(tagIdsByName).To(HaveKey("shared-" + fam))
				Expect(tagIdsByName["shared-"+fam]).To(Equal(*sharedTargetTag.Id))
				Expect(tagIdsByName).To(HaveKey("srconly-" + fam))
				Expect(tagIdsByName["srconly-"+fam]).ToNot(Equal(*sourceOnlyTag.Id))

				patient := getPatient(*target.Id, *uniqueSource.Id)
				Expect(patient.Tags).ToNot(BeNil())
				Expect(*patient.Tags).To(ConsistOf(*sharedTargetTag.Id, tagIdsByName["srconly-"+fam]))
			})

			It("renames colliding sites and reassigns patients", func() {
				req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s", *target.Id), "")
				asServer(req)
				resp := do(req)
				expectStatus(resp, http.StatusOK)
				clinic := decodeAs[client.ClinicV1](resp)

				siteNames := make([]string, 0, len(clinic.Sites))
				for _, site := range clinic.Sites {
					siteNames = append(siteNames, site.Name)
				}
				Expect(siteNames).To(ContainElements("site-"+fam, fmt.Sprintf("site-%s (2)", fam)))

				patient := getPatient(*target.Id, *uniqueSource.Id)
				Expect(patient.Sites).To(HaveLen(1))
				Expect(patient.Sites[0].Name).To(Equal(fmt.Sprintf("site-%s (2)", fam)))
				Expect(patient.Sites[0].Id).ToNot(Equal(targetSite.Id))
			})

			It("deletes the source clinic", func() {
				req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s", *source.Id), "")
				asServer(req)
				resp := do(req)
				expectStatus(resp, http.StatusNotFound)
			})

			It("persists the merge plan", func() {
				// PORT-TO-PG: merge plans have no read endpoint; they are only
				// persisted to the merge_plans collection for auditing.
				sourceObjId, err := primitive.ObjectIDFromHex(*source.Id)
				Expect(err).ToNot(HaveOccurred())
				count, err := test.GetTestDatabase().Collection("merge_plans").CountDocuments(testCtx(), bson.M{
					"type":            "clinic",
					"plan.source._id": sourceObjId,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int64(1)))
			})
		})
	})

	Describe("blocked merges", func() {
		var source, target client.ClinicV1

		BeforeAll(func() {
			fam := uniqueId()
			source = createClinic(admin)
			target = createClinic(admin)

			mrn := "BLK" + fam
			createCustodialPatient(*source.Id, admin, map[string]interface{}{
				"fullName": "Blocked Source " + fam, "birthDate": "1969-09-09", "mrn": mrn,
			})
			createCustodialPatient(*target.Id, admin, map[string]interface{}{
				"fullName": "Blocked Target " + fam, "birthDate": "1968-08-08", "mrn": mrn,
			})

			req := prepareRequestWithBody(http.MethodPut,
				fmt.Sprintf("/v1/clinics/%s/settings/mrn", *target.Id),
				jsonBody(map[string]interface{}{"required": true, "unique": true}))
			asServer(req)
			expectStatus(do(req), http.StatusOK)
		})

		It("reports duplicate MRNs in the target workspace as blocking errors", func() {
			report := generateReport(*target.Id, *source.Id)
			Expect(report.PreventsMerge()).To(BeTrue())

			var messages []string
			for _, err := range report.PatientPlans.Errors() {
				messages = append(messages, err.Message)
			}
			Expect(messages).To(ContainElement(ContainSubstring("MRN uniqueness error")))
		})

		It("refuses to execute a blocked merge", func() {
			executeMerge(*target.Id, *source.Id, http.StatusBadRequest)
		})
	})
})
