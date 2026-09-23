package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/tidepool-org/clinic/api"
	"github.com/tidepool-org/clinic/client"
)

var _ = Describe("Connection Issues Integration Test", Ordered, func() {
	const endpoint = "/v1/patients/connection_issues"

	var clinic client.ClinicV1
	var patient api.PatientV1

	patientEndpoint := func() string {
		return fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, *patient.Id)
	}

	getPatient := func() api.PatientV1 {
		GinkgoHelper()
		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodGet, patientEndpoint(), "")
		asClinician(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

		fetched := api.PatientV1{}
		Expect(json.NewDecoder(rec.Result().Body).Decode(&fetched)).To(Succeed())
		return fetched
	}

	updateDataSources := func(sources []api.DataSourceV1) {
		GinkgoHelper()
		body, err := json.Marshal(sources)
		Expect(err).ToNot(HaveOccurred())

		rec := httptest.NewRecorder()
		endpoint := fmt.Sprintf("/v1/patients/%s/data_sources", *patient.Id)
		req := prepareRequestWithBody(http.MethodPut, endpoint, bytes.NewReader(body))
		asServer(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
	}

	updateConnectionIssues := func() {
		GinkgoHelper()
		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodPost, endpoint, "")
		asServer(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
	}

	datetime := func(t time.Time) *api.DatetimeV1 {
		value := api.DatetimeV1(t.Format(time.RFC3339))
		return &value
	}

	setHidden := func(hidden bool, as func(*http.Request)) *http.Response {
		GinkgoHelper()
		body, err := json.Marshal(api.ConnectionIssueHiddenV1{Hidden: hidden})
		Expect(err).ToNot(HaveOccurred())

		rec := httptest.NewRecorder()
		endpoint := patientEndpoint() + "/connection_issue/hidden"
		req := prepareRequestWithBody(http.MethodPut, endpoint, bytes.NewReader(body))
		as(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		return rec.Result()
	}

	// listPatients returns the ids of the clinic's patients matching the query string
	listPatients := func(query string) ([]string, int) {
		GinkgoHelper()
		rec := httptest.NewRecorder()
		endpoint := fmt.Sprintf("/v1/clinics/%s/patients?%s", *clinic.Id, query)
		req := prepareRequest(http.MethodGet, endpoint, "")
		asClinician(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		if rec.Result().StatusCode != http.StatusOK {
			return nil, rec.Result().StatusCode
		}

		response := client.PatientsResponseV1{}
		Expect(json.NewDecoder(rec.Result().Body).Decode(&response)).To(Succeed())
		Expect(response.Data).ToNot(BeNil())
		var ids []string
		for _, listed := range *response.Data {
			ids = append(ids, *listed.Id)
		}
		return ids, http.StatusOK
	}

	expectHidden := func(hidden bool) {
		GinkgoHelper()
		Expect(getPatient().ConnectionIssue).ToNot(BeNil())
		Expect(getPatient().ConnectionIssue.Hidden).To(PointTo(Equal(hidden)))
	}

	Describe("Update Connection Issues", func() {
		It("Succeeds for a backend service", func() {
			updateConnectionIssues()
		})

		It("Is forbidden for a clinician", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusForbidden))
		})
	})

	Describe("Create a clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics",
				"./test/common_fixtures/01_create_clinic.json")
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

	Describe("Create a patient with a dexcom connection request", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients", *clinic.Id),
				"./test/common_fixtures/02_create_patient.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
			Expect(json.NewDecoder(rec.Result().Body).Decode(&patient)).To(Succeed())

			rec = httptest.NewRecorder()
			req = prepareRequest(http.MethodPost, patientEndpoint()+"/connect/dexcom", "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
			Expect(getPatient().ConnectionIssue).To(BeNil())
		})
	})

	Describe("Detect connection issues", func() {
		var latest time.Time

		It("Reports stale data", func() {
			latest = time.Now().Add(-72 * time.Hour).UTC().Truncate(time.Second)
			updateDataSources([]api.DataSourceV1{{
				ProviderName:   "dexcom",
				State:          api.Connected,
				CreatedTime:    datetime(time.Now()),
				LatestDataTime: datetime(latest),
			}})

			updateConnectionIssues()

			Expect(getPatient().ConnectionIssue).To(PointTo(MatchAllFields(Fields{
				"Cause":  Equal(api.ConnectionIssueCauseStaleData),
				"Hidden": PointTo(BeFalse()),
			})))
		})

		Describe("Listing by connection issue", func() {
			It("Includes the patient for a matching visible cause", func() {
				ids, status := listPatients("connectionIssueCauses=staleData")
				Expect(status).To(Equal(http.StatusOK))
				Expect(ids).To(ContainElement(*patient.Id))
			})

			It("Excludes the patient for other causes", func() {
				ids, status := listPatients("connectionIssueCauses=error,disconnected")
				Expect(status).To(Equal(http.StatusOK))
				Expect(ids).ToNot(ContainElement(*patient.Id))
			})

			It("Excludes the patient when only hidden issues are requested", func() {
				ids, status := listPatients(
					"connectionIssueCauses=staleData&onlyHiddenConnectionIssues=true")
				Expect(status).To(Equal(http.StatusOK))
				Expect(ids).ToNot(ContainElement(*patient.Id))
			})

			It("Rejects an unknown cause", func() {
				_, status := listPatients("connectionIssueCauses=staleData,bogus")
				Expect(status).To(Equal(http.StatusBadRequest))
			})
		})

		Describe("Hiding the issue", func() {
			It("Is hidden by a clinician", func() {
				resp := setHidden(true, asClinician)
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				updated := api.PatientV1{}
				Expect(json.NewDecoder(resp.Body).Decode(&updated)).To(Succeed())
				Expect(updated.ConnectionIssue.Hidden).To(PointTo(BeTrue()))
				expectHidden(true)
			})

			It("Is excluded from the visible list and included in the hidden list", func() {
				ids, status := listPatients("connectionIssueCauses=staleData")
				Expect(status).To(Equal(http.StatusOK))
				Expect(ids).ToNot(ContainElement(*patient.Id))

				ids, status = listPatients(
					"connectionIssueCauses=staleData&onlyHiddenConnectionIssues=true")
				Expect(status).To(Equal(http.StatusOK))
				Expect(ids).To(ContainElement(*patient.Id))

				ids, status = listPatients("onlyHiddenConnectionIssues=true")
				Expect(status).To(Equal(http.StatusOK))
				Expect(ids).To(ContainElement(*patient.Id))
			})

			It("Is unhidden by a backend service", func() {
				resp := setHidden(false, asServer)
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				expectHidden(false)
			})

			It("Stays hidden while the cause is unchanged", func() {
				Expect(setHidden(true, asClinician).StatusCode).To(Equal(http.StatusOK))

				updateConnectionIssues()

				expectHidden(true)
			})
		})

		It("Reports an error", func() {
			updateDataSources([]api.DataSourceV1{{
				ProviderName:   "dexcom",
				State:          api.Error,
				CreatedTime:    datetime(time.Now()),
				LatestDataTime: datetime(latest),
			}})

			updateConnectionIssues()

			Expect(getPatient().ConnectionIssue).To(PointTo(MatchAllFields(Fields{
				"Cause":  Equal(api.ConnectionIssueCauseError),
				"Hidden": PointTo(BeFalse()),
			})))
		})

		It("Clears the issue once the source is connected with fresh data", func() {
			updateDataSources([]api.DataSourceV1{{
				ProviderName:   "dexcom",
				State:          api.Connected,
				CreatedTime:    datetime(time.Now()),
				LatestDataTime: datetime(time.Now()),
			}})

			updateConnectionIssues()

			Expect(getPatient().ConnectionIssue).To(BeNil())
		})

		It("Cannot be hidden once there is no issue", func() {
			Expect(setHidden(true, asClinician).StatusCode).To(Equal(http.StatusNotFound))
		})
	})
})
