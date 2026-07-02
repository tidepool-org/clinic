package integration_test

import (
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/client"
)

// Pins site management at the HTTP level (the service-level coverage lives in
// patientsites_test.go): CRUD, case-insensitive name uniqueness, the 50 site
// limit, denormalized site renames on patients, delete cascades and merges.
var _ = Describe("Clinic Sites", Ordered, func() {
	var auth func(*http.Request)
	var clinicId string

	patientSites := func(patientId string) map[string]string {
		GinkgoHelper()
		patient := getPatient(clinicId, patientId)
		sites := map[string]string{}
		for _, s := range patient.Sites {
			sites[s.Id] = string(s.Name)
		}
		return sites
	}

	clinicSiteNames := func() map[string]string {
		GinkgoHelper()
		clinic := fetchClinic(clinicId)
		sites := map[string]string{}
		for _, s := range clinic.Sites {
			sites[s.Id] = string(s.Name)
		}
		return sites
	}

	BeforeAll(func() {
		admin := newStubUser()
		auth = asUser(admin.UserID)
		clinicId = *createClinic(auth).Id
	})

	Describe("site lifecycle", func() {
		var site client.SiteV1
		var siteName string
		var patient client.PatientV1

		It("creates a site visible on the clinic", func() {
			siteName = "site-" + uniqueId()
			site = createSite(clinicId, siteName, auth)
			Expect(site.Id).ToNot(BeEmpty())
			Expect(clinicSiteNames()).To(HaveKeyWithValue(site.Id, siteName))
		})

		It("rejects duplicate site names case-insensitively", func() {
			for _, name := range []string{siteName, strings.ToUpper(siteName)} {
				req := prepareRequestWithBody(http.MethodPost,
					fmt.Sprintf("/v1/clinics/%s/sites", clinicId),
					jsonBody(map[string]interface{}{"name": name}))
				auth(req)
				expectStatus(do(req), http.StatusConflict)
			}
		})

		It("renames a site on the clinic and on assigned patients", func() {
			patient = createCustodialPatient(clinicId, auth, map[string]interface{}{
				"sites": []map[string]interface{}{{"id": site.Id, "name": siteName}},
			})
			Expect(patientSites(*patient.Id)).To(HaveKeyWithValue(site.Id, siteName))

			siteName = "renamed-" + uniqueId()
			req := prepareRequestWithBody(http.MethodPut,
				fmt.Sprintf("/v1/clinics/%s/sites/%s", clinicId, site.Id),
				jsonBody(map[string]interface{}{"id": site.Id, "name": siteName}))
			auth(req)
			expectStatus(do(req), http.StatusOK)

			// The site name is denormalized onto patients and must be updated
			// everywhere.
			Expect(clinicSiteNames()).To(HaveKeyWithValue(site.Id, siteName))
			Expect(patientSites(*patient.Id)).To(HaveKeyWithValue(site.Id, siteName))
		})

		It("deletes a site from the clinic and from assigned patients", func() {
			req := prepareRequest(http.MethodDelete,
				fmt.Sprintf("/v1/clinics/%s/sites/%s", clinicId, site.Id), "")
			auth(req)
			expectStatus(do(req), http.StatusNoContent)

			Expect(clinicSiteNames()).ToNot(HaveKey(site.Id))
			Expect(patientSites(*patient.Id)).To(BeEmpty())
		})
	})

	Describe("site merge", func() {
		var source, target client.SiteV1
		var inSource, inBoth, inTarget client.PatientV1

		BeforeAll(func() {
			source = createSite(clinicId, "source-"+uniqueId(), auth)
			target = createSite(clinicId, "target-"+uniqueId(), auth)

			bodyWith := func(sites ...client.SiteV1) map[string]interface{} {
				siteRefs := []map[string]interface{}{}
				for _, s := range sites {
					siteRefs = append(siteRefs, map[string]interface{}{"id": s.Id, "name": s.Name})
				}
				return map[string]interface{}{"sites": siteRefs}
			}

			inSource = createCustodialPatient(clinicId, auth, bodyWith(source))
			inBoth = createCustodialPatient(clinicId, auth, bodyWith(source, target))
			inTarget = createCustodialPatient(clinicId, auth, bodyWith(target))
		})

		It("is only allowed for backend services", func() {
			// Site merges have no clinician policy rule; even clinic admins
			// are rejected.
			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/sites/%s/merge", clinicId, target.Id),
				jsonBody(map[string]interface{}{"id": source.Id}))
			auth(req)
			expectStatus(do(req), http.StatusForbidden)
		})

		It("rejects merging a site into itself", func() {
			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/sites/%s/merge", clinicId, target.Id),
				jsonBody(map[string]interface{}{"id": target.Id}))
			asServer(req)
			expectStatus(do(req), http.StatusBadRequest)
		})

		It("returns 404 when the target site does not exist", func() {
			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/sites/%s/merge", clinicId, "ffffffffffffffffffffffff"),
				jsonBody(map[string]interface{}{"id": source.Id}))
			asServer(req)
			expectStatus(do(req), http.StatusNotFound)
		})

		It("moves the source site's patients to the target without duplicates", func() {
			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/sites/%s/merge", clinicId, target.Id),
				jsonBody(map[string]interface{}{"id": source.Id}))
			asServer(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			// The merge endpoint serializes the domain site shape, which
			// exposes the patient count as "patients" rather than the
			// spec's "numPatients".
			merged := decodeAs[struct {
				Id       string `json:"id"`
				Name     string `json:"name"`
				Patients int    `json:"patients"`
			}](resp)
			Expect(merged.Id).To(Equal(target.Id))
			Expect(merged.Patients).To(Equal(3))

			Expect(clinicSiteNames()).ToNot(HaveKey(source.Id))

			Expect(patientSites(*inSource.Id)).To(Equal(map[string]string{target.Id: string(target.Name)}))
			// The patient assigned to both sites ends up with a single membership.
			Expect(patientSites(*inBoth.Id)).To(Equal(map[string]string{target.Id: string(target.Name)}))
			Expect(patientSites(*inTarget.Id)).To(Equal(map[string]string{target.Id: string(target.Name)}))
		})
	})

	Describe("limits", func() {
		It("rejects creating more than 50 sites", func() {
			limitAuth := asUser(newStubUser().UserID)
			limitClinicId := *createClinic(limitAuth).Id
			for i := 0; i < 50; i++ {
				createSite(limitClinicId, fmt.Sprintf("limit-%02d-%s", i, uniqueId()), limitAuth)
			}

			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/sites", limitClinicId),
				jsonBody(map[string]interface{}{"name": "toomany-" + uniqueId()}))
			limitAuth(req)
			expectStatus(do(req), http.StatusUnprocessableEntity)
		})
	})
})
