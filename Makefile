# Clinic Makefile

# Generates server files
generate:
	swagger-cli bundle ../TidepoolApi/reference/clinic.v1.yaml -o ./spec/clinic.v1.yaml -t yaml
	oapi-codegen -exclude-tags=confirmation -package=api -generate=server spec/clinic.v1.yaml > api/gen_server.go
	oapi-codegen -exclude-tags=confirmation -package=api -generate=spec spec/clinic.v1.yaml > api/gen_spec.go
	oapi-codegen -exclude-tags=confirmation -package=api -generate=types spec/clinic.v1.yaml > api/gen_types.go
	oapi-codegen -exclude-tags=confirmation -package=api -generate=types spec/clinic.v1.yaml > client/types.go
	oapi-codegen -exclude-tags=confirmation -package=api -generate=client spec/clinic.v1.yaml > client/client.go
	go generate ./...

# Runs tests
test:
	./test.sh

# Builds package
build:
	./build.sh
