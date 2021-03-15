module github.com/tidepool-org/clinic

go 1.15

require (
	github.com/deepmap/oapi-codegen v1.5.1
	github.com/getkin/kin-openapi v0.37.0
	github.com/golang/mock v1.5.0 // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/labstack/echo/v4 v4.1.17
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/tidepool-org/clinic/client v0.0.0-00010101000000-000000000000
	github.com/tidepool-org/go-common v0.7.1
	github.com/tidepool-org/oapi-codegen v1.3.9-0.20200610000610-300bfbd05ff1
	go.mongodb.org/mongo-driver v1.3.2
	go.uber.org/fx v1.13.1
)

replace github.com/tidepool-org/clinic/client => ./client
