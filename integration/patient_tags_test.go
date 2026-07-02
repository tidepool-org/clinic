package integration_test

import (
	"fmt"
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// fetchClinic returns the clinic DTO, which includes patientTags, sites and
// lastDeletedPatientTag. Shared by the tag and site specs.
func fetchClinic(clinicId string) client.ClinicV1 {
	GinkgoHelper()
	req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s", clinicId), "")
	asServer(req)
	resp := do(req)
	expectStatus(resp, http.StatusOK)
	return decodeAs[client.ClinicV1](resp)
}

func clinicTagNames(clinic client.ClinicV1) []string {
	names := []string{}
	if clinic.PatientTags != nil {
		for _, t := range *clinic.PatientTags {
			names = append(names, t.Name)
		}
	}
	return names
}

// Pins patient tag management: CRUD, uniqueness, the 50 tag limit, bulk
// assignment/removal (empty body targets every clinic patient), cascade
// deletion from patients, and tag to site conversion.
var _ = Describe("Patient Tags", Ordered, func() {
	var auth func(*http.Request)
	var memberAuth func(*http.Request)
	var clinicId string
	var p1, p2, p3 client.PatientV1

	patientTagIds := func(patientId string) []string {
		GinkgoHelper()
		patient := getPatient(clinicId, patientId)
		if patient.Tags == nil {
			return []string{}
		}
		return *patient.Tags
	}

	BeforeAll(func() {
		admin := newStubUser()
		auth = asUser(admin.UserID)
		clinicId = *createClinic(auth).Id

		member := newStubUser()
		createClinicianDirect(clinicId, member.UserID, "CLINIC_MEMBER")
		memberAuth = asUser(member.UserID)

		p1 = createCustodialPatient(clinicId, auth, nil)
		p2 = createCustodialPatient(clinicId, auth, nil)
		p3 = createCustodialPatient(clinicId, auth, nil)
	})

	Describe("tag lifecycle", func() {
		var tag client.PatientTagV1
		var tagName string

		It("creates a tag visible on the clinic", func() {
			tagName = "tag-" + uniqueId()
			tag = createPatientTag(clinicId, tagName, auth)
			Expect(tag.Id).ToNot(BeNil())
			Expect(tag.Name).To(Equal(tagName))
			Expect(clinicTagNames(fetchClinic(clinicId))).To(ContainElement(tagName))
		})

		It("rejects duplicate tag names", func() {
			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patient_tags", clinicId),
				jsonBody(map[string]interface{}{"name": tagName}))
			auth(req)
			expectStatus(do(req), http.StatusConflict)
		})

		It("renames a tag", func() {
			tagName = "renamed-" + uniqueId()
			req := prepareRequestWithBody(http.MethodPut,
				fmt.Sprintf("/v1/clinics/%s/patient_tags/%s", clinicId, *tag.Id),
				jsonBody(map[string]interface{}{"name": tagName}))
			auth(req)
			expectStatus(do(req), http.StatusOK)
			Expect(clinicTagNames(fetchClinic(clinicId))).To(ContainElement(tagName))
		})

		It("rejects renaming to an existing tag's name", func() {
			other := createPatientTag(clinicId, "other-"+uniqueId(), auth)
			req := prepareRequestWithBody(http.MethodPut,
				fmt.Sprintf("/v1/clinics/%s/patient_tags/%s", clinicId, *other.Id),
				jsonBody(map[string]interface{}{"name": tagName}))
			auth(req)
			expectStatus(do(req), http.StatusConflict)
		})

		It("rejects deletion by clinic members without write access", func() {
			req := prepareRequest(http.MethodDelete,
				fmt.Sprintf("/v1/clinics/%s/patient_tags/%s", clinicId, *tag.Id), "")
			memberAuth(req)
			expectStatus(do(req), http.StatusForbidden)
		})

		It("deletes a tag from the clinic and all tagged patients", func() {
			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients/assign_tag/%s", clinicId, *tag.Id),
				jsonBody([]string{*p1.Id}))
			auth(req)
			expectStatus(do(req), http.StatusOK)
			Expect(patientTagIds(*p1.Id)).To(ContainElement(*tag.Id))

			req = prepareRequest(http.MethodDelete,
				fmt.Sprintf("/v1/clinics/%s/patient_tags/%s", clinicId, *tag.Id), "")
			auth(req)
			expectStatus(do(req), http.StatusNoContent)

			clinic := fetchClinic(clinicId)
			Expect(clinicTagNames(clinic)).ToNot(ContainElement(tagName))
			// Tag deletion does NOT cascade to patients in this service: the
			// dangling tag id stays on the patient document and is cleaned up
			// out-of-band (the worker reacts to lastDeletedPatientTag). The
			// Postgres port must preserve or replace that mechanism.
			Expect(patientTagIds(*p1.Id)).To(ContainElement(*tag.Id))

			// PORT-TO-PG: lastDeletedPatientTag is persisted (as the bare tag
			// object id) for the worker but not exposed through the API.
			db := test.GetTestDatabase()
			clinicObjId, err := primitive.ObjectIDFromHex(clinicId)
			Expect(err).ToNot(HaveOccurred())
			var doc struct {
				LastDeletedPatientTag primitive.ObjectID `bson:"lastDeletedPatientTag"`
			}
			Expect(db.Collection("clinics").
				FindOne(testCtx(), bson.M{"_id": clinicObjId}).
				Decode(&doc)).To(Succeed())
			Expect(doc.LastDeletedPatientTag.Hex()).To(Equal(*tag.Id))
		})
	})

	Describe("bulk assignment", func() {
		var tag client.PatientTagV1

		BeforeAll(func() {
			tag = createPatientTag(clinicId, "bulk-"+uniqueId(), auth)
		})

		It("assigns a tag to the provided patients only", func() {
			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients/assign_tag/%s", clinicId, *tag.Id),
				jsonBody([]string{*p1.Id, *p2.Id}))
			auth(req)
			expectStatus(do(req), http.StatusOK)

			Expect(patientTagIds(*p1.Id)).To(ContainElement(*tag.Id))
			Expect(patientTagIds(*p2.Id)).To(ContainElement(*tag.Id))
			Expect(patientTagIds(*p3.Id)).ToNot(ContainElement(*tag.Id))
		})

		It("removes a tag from the provided patients only", func() {
			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients/delete_tag/%s", clinicId, *tag.Id),
				jsonBody([]string{*p1.Id}))
			auth(req)
			expectStatus(do(req), http.StatusOK)

			Expect(patientTagIds(*p1.Id)).ToNot(ContainElement(*tag.Id))
			Expect(patientTagIds(*p2.Id)).To(ContainElement(*tag.Id))
		})

		It("assigns a tag to every clinic patient when the body is empty", func() {
			req := prepareRequest(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients/assign_tag/%s", clinicId, *tag.Id), "")
			auth(req)
			expectStatus(do(req), http.StatusOK)

			for _, id := range []string{*p1.Id, *p2.Id, *p3.Id} {
				Expect(patientTagIds(id)).To(ContainElement(*tag.Id))
			}
		})

		It("removes a tag from every clinic patient when the body is empty", func() {
			req := prepareRequest(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients/delete_tag/%s", clinicId, *tag.Id), "")
			auth(req)
			expectStatus(do(req), http.StatusOK)

			for _, id := range []string{*p1.Id, *p2.Id, *p3.Id} {
				Expect(patientTagIds(id)).ToNot(ContainElement(*tag.Id))
			}
		})
	})

	Describe("tag to site conversion", func() {
		It("is only allowed for backend services", func() {
			tag := createPatientTag(clinicId, "authz-"+uniqueId(), auth)
			req := prepareRequest(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patient_tags/%s/site", clinicId, *tag.Id), "")
			auth(req)
			expectStatus(do(req), http.StatusForbidden)
		})

		It("converts a tag into a site and re-associates patients", func() {
			name := "conv-" + uniqueId()
			tag := createPatientTag(clinicId, name, auth)

			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients/assign_tag/%s", clinicId, *tag.Id),
				jsonBody([]string{*p1.Id, *p2.Id}))
			auth(req)
			expectStatus(do(req), http.StatusOK)

			req = prepareRequest(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patient_tags/%s/site", clinicId, *tag.Id), "")
			asServer(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			site := decodeAs[client.SiteV1](resp)
			Expect(site.Name).To(Equal(name))

			clinic := fetchClinic(clinicId)
			Expect(clinicTagNames(clinic)).ToNot(ContainElement(name))

			for _, id := range []string{*p1.Id, *p2.Id} {
				Expect(patientTagIds(id)).ToNot(ContainElement(*tag.Id))
				patient := getPatient(clinicId, id)
				siteIds := []string{}
				for _, s := range patient.Sites {
					siteIds = append(siteIds, s.Id)
				}
				Expect(siteIds).To(ContainElement(site.Id))
			}

			// The new site is usable as a list filter.
			response := listPatients(clinicId, url.Values{"sites": {site.Id}}, auth)
			Expect(response.Meta.Count).To(PointTo(Equal(2)))
		})

		It("renames the site with a numeric suffix when a site with the tag's name exists", func() {
			name := "collide-" + uniqueId()
			createSite(clinicId, name, auth)
			tag := createPatientTag(clinicId, name, auth)

			req := prepareRequest(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patient_tags/%s/site", clinicId, *tag.Id), "")
			asServer(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			site := decodeAs[client.SiteV1](resp)
			Expect(site.Name).To(Equal(name + " (2)"))
		})

		It("returns 404 for a tag that does not exist", func() {
			req := prepareRequest(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patient_tags/%s/site", clinicId, "ffffffffffffffffffffffff"), "")
			asServer(req)
			expectStatus(do(req), http.StatusNotFound)
		})
	})

	Describe("limits", func() {
		It("rejects creating more than 50 tags", func() {
			limitAuth := asUser(newStubUser().UserID)
			limitClinicId := *createClinic(limitAuth).Id
			for i := 0; i < 50; i++ {
				createPatientTag(limitClinicId, fmt.Sprintf("limit-%02d-%s", i, uniqueId()), limitAuth)
			}

			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patient_tags", limitClinicId),
				jsonBody(map[string]interface{}{"name": "toomany-" + uniqueId()}))
			limitAuth(req)
			expectStatus(do(req), http.StatusUnprocessableEntity)
		})

		It("rejects tag conversion when the clinic already has 50 sites", func() {
			limitAuth := asUser(newStubUser().UserID)
			limitClinicId := *createClinic(limitAuth).Id
			for i := 0; i < 50; i++ {
				createSite(limitClinicId, fmt.Sprintf("site-%02d-%s", i, uniqueId()), limitAuth)
			}
			tag := createPatientTag(limitClinicId, "overflow-"+uniqueId(), limitAuth)

			req := prepareRequest(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patient_tags/%s/site", limitClinicId, *tag.Id), "")
			asServer(req)
			expectStatus(do(req), http.StatusConflict)
		})
	})
})
