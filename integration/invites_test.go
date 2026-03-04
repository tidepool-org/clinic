package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/client"
)

var _ = Describe("Clinician Invites", Ordered, func() {
	var clinic client.ClinicV1
	var inviteId string
	var secondInviteId string

	Describe("Create clinic for invite tests", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/invites_fixtures/01_create_clinic.json")
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

	Describe("Create an invited clinician", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/clinicians", *clinic.Id), "./test/invites_fixtures/02_create_invite.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var created client.ClinicianV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &created)).To(Succeed())
			Expect(created.InviteId).ToNot(BeNil())
			inviteId = *created.InviteId
		})
	})

	Describe("Get invited clinician by invite ID", func() {
		It("Returns the invited clinician", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/invites/clinicians/%s/clinician", *clinic.Id, inviteId), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var invited client.ClinicianV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &invited)).To(Succeed())
			Expect(invited.InviteId).ToNot(BeNil())
			Expect(*invited.InviteId).To(Equal(inviteId))
		})
	})

	Describe("Associate invited clinician to a user", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPatch, fmt.Sprintf("/v1/clinics/%s/invites/clinicians/%s/clinician", *clinic.Id, inviteId), "./test/invites_fixtures/03_associate_invite.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var associated client.ClinicianV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &associated)).To(Succeed())
			Expect(associated.Id).ToNot(BeNil())
			Expect(*associated.Id).To(Equal("2345678901"))
		})
	})

	Describe("Verify clinician now appears in clinician list", func() {
		It("Returns the associated clinician", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians", *clinic.Id), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var clinicians client.CliniciansV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &clinicians)).To(Succeed())

			found := false
			for _, c := range clinicians {
				if c.Id != nil && *c.Id == "2345678901" {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	Describe("Create another invite and delete it", func() {
		It("Creates the second invite", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/clinicians", *clinic.Id), "./test/invites_fixtures/04_create_second_invite.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var created client.ClinicianV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &created)).To(Succeed())
			Expect(created.InviteId).ToNot(BeNil())
			secondInviteId = *created.InviteId
		})

		It("Deletes the invite", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/clinics/%s/invites/clinicians/%s/clinician", *clinic.Id, secondInviteId), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Verify deleted invite returns 404", func() {
		It("Returns 404", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/invites/clinicians/%s/clinician", *clinic.Id, secondInviteId), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNotFound))
		})
	})
})
