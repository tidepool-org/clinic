package integration_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Device Issues Integration Test", func() {
	triggerCheck := func(authenticate func(*http.Request)) int {
		GinkgoHelper()

		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodPost, "/v1/device_issues", "")
		authenticate(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		return rec.Result().StatusCode
	}

	It("Succeeds for a backend service", func() {
		Expect(triggerCheck(asServer)).To(Equal(http.StatusNoContent))
	})

	It("Is forbidden for a clinician", func() {
		Expect(triggerCheck(asClinician)).To(Equal(http.StatusForbidden))
	})
})
