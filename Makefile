# Clinic Makefile

# Generates server files
generate_grpc:
	rm -rf generated
	openapi-generator generate -i clinic.fixed.v1.yaml -g protobuf-schema -o generated --package-name clinic
	cd generated; protoc --go_out=plugins=grpc,paths=source_relative:. models/*
	cd generated; protoc --go_out=plugins=grpc,paths=source_relative:. services/default_service.proto
	python cmd/fixProtoForGw.py
	cd generated; protoc -I=. -I `go list -m -f "{{.Dir}}" github.com/grpc-ecosystem/grpc-gateway/v2`/third_party/googleapis --grpc-gateway_out=. services/default_service.proto
	sed  -i .bak 's/"models"/"github.com\/tidepool-org\/clinic\/generated\/models"/' generated/services/default_service.pb.go; rm generated/services/default_service.pb.go.bak

# Runs tests
test:
	./test.sh

# Builds package
build:
	./build.sh

generate_yaml:
	python cmd/fixYaml.py

generate_gw_models: generate_yaml
	go run ../oapi-codegen/cmd/oapi-codegen/oapi-codegen.go  -generate=types clinic.fixed.v1.yaml > api/gen_types.go
	sed  -i .bak 's/package Clinic/package api/' api/gen_types.go; rm api/gen_types.go.bak

generate_gw_spec: generate_yaml
	go run ../oapi-codegen/cmd/oapi-codegen/oapi-codegen.go  -generate=spec clinic.fixed.v1.yaml > api/gen_spec.go
	sed  -i .bak 's/package Clinic/package api/' api/gen_spec.go; rm api/gen_spec.go.bak

generate_gw: generate_gw_models generate_gw_spec

generate_policy:
	python cmd/createPolicyfile.py

generate: generate_gw generate_grpc generate_policy