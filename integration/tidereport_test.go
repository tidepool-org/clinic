package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic/client"
)

var _ = Describe("TIDE Report Integration Test", Ordered, func() {
	var clinic client.ClinicV1

	tideReport := func(query url.Values) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		endpoint := fmt.Sprintf("/v1/clinics/%s/tide_report?%s", *clinic.Id, query.Encode())
		req := prepareRequest(http.MethodGet, endpoint, "")
		asClinician(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		return rec
	}

	tideQuery := func() url.Values {
		query := url.Values{}
		query.Set("period", "7d")
		query.Set("lastDataCutoff", time.Now().Add(-14*24*time.Hour).UTC().Format(time.RFC3339))
		return query
	}

	Describe("Create a clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/common_fixtures/01_create_clinic.json")
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

	Describe("Retrieve a TIDE report", func() {
		It("Succeeds without a tags parameter", func() {
			rec := tideReport(tideQuery())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var report client.TideResponseV1
			Expect(json.NewDecoder(rec.Result().Body).Decode(&report)).To(Succeed())
			Expect(report.Config.Tags).To(BeNil())

		})

		It("Succeeds with an empty tags parameter", func() {
			query := tideQuery()
			query.Set("tags", "")
			rec := tideReport(query)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		When("no sites are provided", func() {
			It("has no sites in the returned config", func() {
				rec := tideReport(tideQuery())
				Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

				var report client.TideResponseV1
				Expect(json.NewDecoder(rec.Result().Body).Decode(&report)).To(Succeed())
				Expect(report.Config.Sites).To(BeNil())
			})
		})
	})
})
