#!/bin/sh -eu

rm -rf dist
mkdir dist
go build -o dist/clinic clinic.go
go build -o dist/xealth cmd/xealth/main.go

