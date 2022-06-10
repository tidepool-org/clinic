module github.com/tidepool-org/clinic

go 1.16

require (
	github.com/deepmap/oapi-codegen v1.10.0
	github.com/fatih/structs v1.1.0
	github.com/getkin/kin-openapi v0.94.0
	github.com/globalsign/mgo v0.0.0-20181015135952-eeefdecb41b8
	github.com/golang/mock v1.6.0
	github.com/hashicorp/golang-lru v0.5.3
	github.com/jaswdr/faker v1.4.2
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/labstack/echo/v4 v4.7.2
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/open-policy-agent/opa v0.40.0
	github.com/tidepool-org/clinic/client v0.0.0-00010101000000-000000000000
	github.com/tidepool-org/go-common v0.8.3-0.20210528114116-26ab9a2d32b5
	go.mongodb.org/mongo-driver v1.5.1
	go.uber.org/fx v1.13.1
	go.uber.org/zap v1.17.0
)

replace github.com/tidepool-org/clinic/client => ./client
