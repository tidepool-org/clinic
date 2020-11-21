python cmd/fixYaml.py

go run ../oapi-codegen/cmd/oapi-codegen/oapi-codegen.go  -generate=server clinic.fixed.v1.yaml > api/gen_server.go
go run ../oapi-codegen/cmd/oapi-codegen/oapi-codegen.go  -generate=types clinic.fixed.v1.yaml > api/gen_types.go
go run ../oapi-codegen/cmd/oapi-codegen/oapi-codegen.go  -generate=spec clinic.fixed.v1.yaml > api/gen_spec.go
go run ../oapi-codegen/cmd/oapi-codegen/oapi-codegen.go  -generate=client clinic.fixed.v1.yaml > api/gen_client.go


sed  -i .bak 's/package Clinic/package api/' api/gen_types.go; rm api/gen_types.go.bak
sed  -i .bak 's/package Clinic/package api/' api/gen_spec.go; rm api/gen_spec.go.bak
sed  -i .bak 's/package Clinic/package api/' api/gen_server.go; rm api/gen_server.go.bak
sed  -i .bak 's/package Clinic/package api/' api/gen_client.go; rm api/gen_client.go.bak

python cmd/createPolicyfile.py
