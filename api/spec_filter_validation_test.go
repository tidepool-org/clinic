package api_test

import (
	"context"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic/api"
)

var _ = DescribeTable("embedded spec validation of patients list filters",
	func(query string, expectValid bool) {
		swagger, err := api.GetSwagger()
		Expect(err).ToNot(HaveOccurred())
		swagger.Servers = nil

		router, err := gorillamux.NewRouter(swagger)
		Expect(err).ToNot(HaveOccurred())

		req, err := http.NewRequest(http.MethodGet, "/v1/clinics/6066fbabc6f484277200ac64/patients?"+query, nil)
		Expect(err).ToNot(HaveOccurred())

		route, pathParams, err := router.FindRoute(req)
		Expect(err).ToNot(HaveOccurred())

		err = openapi3filter.ValidateRequest(context.Background(), &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
			Options:    &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		})
		if expectValid {
			Expect(err).ToNot(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
	Entry("four decimal places", "cgm.timeCGMUsePercent=%3E%3D0.6955", true),
	Entry("four decimal places without integer digit", "cgm.timeCGMUsePercent=%3E.6955", true),
	Entry("ten decimal places", "cgm.timeCGMUsePercent=%3C0.1234567891", true),
	Entry("eleven decimal places", "cgm.timeCGMUsePercent=%3E%3D0.12345678901", false),
	Entry("ten integer digits", "cgm.averageGlucoseMmol=%3C1234567890.5", true),
	Entry("eleven integer digits", "cgm.averageGlucoseMmol=%3C12345678901.5", false),
	Entry("integer filter", "cgm.timeCGMUseRecords=%3E%3D7", true),
)
