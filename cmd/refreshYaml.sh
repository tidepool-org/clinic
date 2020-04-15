python cmd/fixYaml

go run ~/workspace/opensource/oapi-codegen/cmd/oapi-codegen/oapi-codegen.go  -generate=server clinic.bundled.y1.yaml > api/gen_server.go
go run ~/workspace/opensource/oapi-codegen/cmd/oapi-codegen/oapi-codegen.go  -generate=types clinic.bundled.y1.yaml > api/gen_types.go
go run ~/workspace/opensource/oapi-codegen/cmd/oapi-codegen/oapi-codegen.go  -generate=spec clinic.bundled.y1.yaml > api/gen_spec.go
