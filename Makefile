# Clinic Makefile

# Generates server files
generate:
	swagger-cli bundle ../TidepoolApi/reference/clinic.v1.yaml -o ./spec/clinic.v1.yaml -t yaml
	oapi-codegen -exclude-tags=Confirmations -package=api -generate=server spec/clinic.v1.yaml > api/gen_server.go
	oapi-codegen -exclude-tags=Confirmations -package=api -generate=spec spec/clinic.v1.yaml > api/gen_spec.go
	oapi-codegen -exclude-tags=Confirmations -package=api -generate=types spec/clinic.v1.yaml > api/gen_types.go
	oapi-codegen -exclude-tags=Confirmations -package=api -generate=types spec/clinic.v1.yaml > client/types.go
	oapi-codegen -exclude-tags=Confirmations -package=api -generate=client spec/clinic.v1.yaml > client/client.go
	go generate ./...
	cd client && go generate ./...

# Set flags
go-flags:
	go env -w GOFLAGS=-mod=mod

ginkgo:
ifeq ($(shell which ginkgo),)
	go install github.com/onsi/ginkgo/ginkgo
endif

# Runs tests
test: go-flags ginkgo
	ginkgo -requireSuite -slowSpecThreshold=10 --compilers=2 -r -randomizeSuites -randomizeAllSpecs -succinct -failOnPending -trace -race -progress -keepGoing ./...

# Builds package
build: go-flags
	./build.sh
