package integration_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Connection Issues Integration Test", func() {
	Describe("Update Connection Issues", func() {
		const endpoint = "/v1/patients/connection_issues"

		It("Succeeds for a backend service", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, endpoint, "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
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
})
