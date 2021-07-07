module github.com/tidepool-org/clinic

go 1.16

require (
	github.com/deepmap/oapi-codegen v1.5.1
	github.com/fatih/structs v1.1.0
	github.com/getkin/kin-openapi v0.37.0
	github.com/golang/mock v1.5.0
	github.com/jaswdr/faker v1.4.2
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/labstack/echo/v4 v4.1.17
	github.com/mattn/go-isatty v0.0.13 // indirect
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/open-policy-agent/opa v0.27.1
	github.com/tidepool-org/clinic/client v0.0.0-00010101000000-000000000000
	github.com/tidepool-org/go-common v0.8.3-0.20210528114116-26ab9a2d32b5
	go.mongodb.org/mongo-driver v1.3.2
	go.uber.org/fx v1.13.1
	go.uber.org/zap v1.13.0
	golang.org/x/crypto v0.0.0-20210616213533-5ff15b29337e // indirect
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e // indirect
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c // indirect
)

replace github.com/tidepool-org/clinic/client => ./client
