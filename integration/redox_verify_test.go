package integration_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Redox Verification", Ordered, func() {
	Describe("Verify Redox endpoint", func() {
		It("Returns the challenge back", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/redox/verify", "./test/redox_fixtures/06_verify_request.json")

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var result map[string]string
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &result)).To(Succeed())
			Expect(result["challenge"]).To(Equal("integration-test-challenge"))
		})
	})
})
