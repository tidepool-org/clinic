package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/client"
)

var _ = Describe("Sites", Ordered, func() {
	var clinic client.ClinicV1
	var siteAId string
	var siteBId string
	var tagId string

	Describe("Create clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/sites_fixtures/01_create_clinic.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &clinic)).To(Succeed())
			Expect(clinic.Id).ToNot(BeNil())
		})
	})

	Describe("Create site A", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/sites", *clinic.Id), "./test/sites_fixtures/02_create_site.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var site client.SiteV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &site)).To(Succeed())
			siteAId = string(site.Id)
			Expect(siteAId).ToNot(BeEmpty())
		})
	})

	Describe("Create site B", func() {
		It("Succeeds", func() {
			body := `{"name": "Site Beta"}`
			rec := httptest.NewRecorder()
			req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/sites", *clinic.Id), strings.NewReader(body))
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var site client.SiteV1
			respBody, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(respBody, &site)).To(Succeed())
			siteBId = string(site.Id)
			Expect(siteBId).ToNot(BeEmpty())
		})
	})

	Describe("Verify sites in clinic via GetClinic", func() {
		It("Shows sites", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s", *clinic.Id), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var updatedClinic client.ClinicV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &updatedClinic)).To(Succeed())
			Expect(len(updatedClinic.Sites)).To(BeNumerically(">=", 2))
		})
	})

	Describe("Update site A name", func() {
		It("Succeeds", func() {
			body := fmt.Sprintf(`{"id": "%s", "name": "Site Alpha Updated"}`, siteAId)
			rec := httptest.NewRecorder()
			req := prepareRequestWithBody(http.MethodPut, fmt.Sprintf("/v1/clinics/%s/sites/%s", *clinic.Id, siteAId), strings.NewReader(body))
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var site client.SiteV1
			respBody, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(respBody, &site)).To(Succeed())
			Expect(string(site.Name)).To(Equal("Site Alpha Updated"))
		})
	})

	Describe("Create patient tag and convert to site", func() {
		It("Creates tag", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patient_tags", *clinic.Id), "./test/sites_fixtures/04_create_tag.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var tag client.PatientTagV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &tag)).To(Succeed())
			Expect(tag.Id).ToNot(BeNil())
			Expect(tag.Name).To(Equal("Tag To Site"))
			tagId = *tag.Id
		})

		It("Converts tag to site", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patient_tags/%s/site", *clinic.Id, tagId), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var site client.SiteV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &site)).To(Succeed())
			Expect(string(site.Name)).To(Equal("Tag To Site"))
		})
	})

	Describe("Merge site B into site A", func() {
		It("Succeeds", func() {
			body := fmt.Sprintf(`{"id": "%s"}`, siteBId)
			rec := httptest.NewRecorder()
			req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/sites/%s/merge", *clinic.Id, siteAId), strings.NewReader(body))
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Verify merged site gone, target updated", func() {
		It("Site B is gone", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s", *clinic.Id), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var updatedClinic client.ClinicV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &updatedClinic)).To(Succeed())

			for _, s := range updatedClinic.Sites {
				Expect(string(s.Id)).ToNot(Equal(siteBId))
			}
		})
	})

	Describe("Delete remaining site", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/clinics/%s/sites/%s", *clinic.Id, siteAId), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
		})
	})
})
