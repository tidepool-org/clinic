# Clinic Makefile

TOOLS_BIN = tools/bin
NODE_BIN = node_modules/.bin

OAPI_CODEGEN = $(TOOLS_BIN)/oapi-codegen
SWAGGER_CLI = $(NODE_BIN)/swagger-cli

NODE_PKG_SPECS = \
	@apidevtools/swagger-cli@^4.0.4


# Generates server files
.PHONY: generate
generate: $(SWAGGER_CLI) $(OAPI_CODEGEN)
	$(SWAGGER_CLI) bundle ../TidepoolApi/reference/clinic.v1.yaml -o ./spec/clinic.v1.yaml -t yaml
	$(OAPI_CODEGEN) -exclude-tags=Confirmations -package=api -generate=server spec/clinic.v1.yaml > api/gen_server.go
	$(OAPI_CODEGEN) -exclude-tags=Confirmations -package=api -generate=spec spec/clinic.v1.yaml > api/gen_spec.go
	$(OAPI_CODEGEN) -exclude-tags=Confirmations -package=api -generate=types spec/clinic.v1.yaml > api/gen_types.go
	$(OAPI_CODEGEN) -exclude-tags=Confirmations -package=client -generate=types spec/clinic.v1.yaml > client/types.go
	$(OAPI_CODEGEN) -exclude-tags=Confirmations -package=client -generate=client spec/clinic.v1.yaml > client/client.go
	$(SWAGGER_CLI) bundle ../TidepoolApi/reference/redox.v1.yaml -o ./spec/redox.v1.yaml -t yaml
	$(OAPI_CODEGEN) -package=redox_models -generate=types spec/redox.v1.yaml > redox_models/gen_types.go
	$(OAPI_CODEGEN) -include-tags="Orders (Partner)",Webhooks -package=xealth_client -generate=types,skip-prune ../TidepoolApi/reference/xealth.v2.yaml > xealth_client/gen_types.go
	$(OAPI_CODEGEN) -include-tags="Orders (Partner)" -package=xealth_client -generate=client ../TidepoolApi/reference/xealth.v2.yaml > xealth_client/gen_client.go
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

tools/bin/ginkgo:
	GOBIN=$(shell pwd)/$(TOOLS_BIN) go install github.com/onsi/ginkgo/v2/ginkgo@v2.13.2

# Runs tests
.PHONY: test
test: go-flags $(TOOLS_BIN)/ginkgo
	$(TOOLS_BIN)/ginkgo --require-suite --compilers=2 -r --randomize-suites --randomize-all --succinct --fail-on-pending --trace --race --poll-progress-after=10s --poll-progress-interval=20s --keep-going ./...

# Builds package
.PHONY: build
build: go-flags
	./build.sh

$(OAPI_CODEGEN):
	GOBIN=$(shell pwd)/$(TOOLS_BIN) go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@v2.0.0

$(SWAGGER_CLI): npm-tools

.PHONY: npm-tools
npm-tools:
# When using --no-save, any dependencies not included will be deleted, so one
# has to install all the packages all at the same time. But it saves us from
# having to muck with packages.json.
	npm install --no-save --local $(NODE_PKG_SPECS)
