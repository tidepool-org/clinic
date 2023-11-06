# Clinic Makefile

# Generates server files
.PHONY: generate
generate:
	swagger-cli bundle ../TidepoolApi/reference/clinic.v1.yaml -o ./spec/clinic.v1.yaml -t yaml
	oapi-codegen -exclude-tags=Confirmations -package=api -generate=server spec/clinic.v1.yaml > api/gen_server.go
	oapi-codegen -exclude-tags=Confirmations -package=api -generate=spec spec/clinic.v1.yaml > api/gen_spec.go
	oapi-codegen -exclude-tags=Confirmations -package=api -generate=types spec/clinic.v1.yaml > api/gen_types.go
	oapi-codegen -exclude-tags=Confirmations -package=api -generate=types spec/clinic.v1.yaml > client/types.go
	oapi-codegen -exclude-tags=Confirmations -package=api -generate=client spec/clinic.v1.yaml > client/client.go
	swagger-cli bundle ../TidepoolApi/reference/redox.v1.yaml -o ./spec/redox.v1.yaml -t yaml
	oapi-codegen -package=redox_models -generate=types spec/redox.v1.yaml > redox_models/gen_types.go
	go generate ./...
	cd client && go generate ./...

# Generate linkerd service profile
service-profile/profile.yaml: service-profile/clinic.v1.yaml service-profile
	linkerd profile --ignore-cluster --open-api service-profile/clinic.v1.yaml clinic > service-profile/profile.yaml

service-profile/clinic.v1.yaml: generate service-profile
	openapi-filter -f Confirmations --checkTags spec/clinic.v1.yaml service-profile/clinic.v1.yaml

service-profile:
	mkdir -p service-profile

# Set flags
.PHONY: go-flags
go-flags:
	go env -w GOFLAGS=-mod=mod

.PHONY: ginkgo
ginkgo:
ifeq ($(shell which ginkgo),)
	go install github.com/onsi/ginkgo/v2/ginkgo
endif

# Runs tests
.PHONY: test
test: go-flags ginkgo
	ginkgo --require-suite --compilers=2 -r --randomize-suites --randomize-all --succinct --fail-on-pending --trace --race --poll-progress-after=10s --poll-progress-interval=20s --keep-going ./...

# Builds package
.PHONY: build
build: go-flags
	./build.sh
