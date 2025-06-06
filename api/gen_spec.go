// Package api provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/oapi-codegen/oapi-codegen/v2 version v2.4.1 DO NOT EDIT.
package api

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"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",
	"Z4gDxjgg7j7d/f8BAAD//1OBRB0rxgEA",
}

// GetSwagger returns the content of the embedded swagger specification file
// or error if failed to decode
func decodeSpec() ([]byte, error) {
	zipped, err := base64.StdEncoding.DecodeString(strings.Join(swaggerSpec, ""))
	if err != nil {
		return nil, fmt.Errorf("error base64 decoding spec: %w", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %w", err)
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(zr)
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %w", err)
	}

	return buf.Bytes(), nil
}

var rawSpec = decodeSpecCached()

// a naive cached of a decoded swagger spec
func decodeSpecCached() func() ([]byte, error) {
	data, err := decodeSpec()
	return func() ([]byte, error) {
		return data, err
	}
}

// Constructs a synthetic filesystem for resolving external references when loading openapi specifications.
func PathToRawSpec(pathToFile string) map[string]func() ([]byte, error) {
	res := make(map[string]func() ([]byte, error))
	if len(pathToFile) > 0 {
		res[pathToFile] = rawSpec
	}

	return res
}

// GetSwagger returns the Swagger specification corresponding to the generated code
// in this file. The external references of Swagger specification are resolved.
// The logic of resolving external references is tightly connected to "import-mapping" feature.
// Externally referenced files must be embedded in the corresponding golang packages.
// Urls can be supported but this task was out of the scope.
func GetSwagger() (swagger *openapi3.T, err error) {
	resolvePath := PathToRawSpec("")

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	loader.ReadFromURIFunc = func(loader *openapi3.Loader, url *url.URL) ([]byte, error) {
		pathToFile := url.String()
		pathToFile = path.Clean(pathToFile)
		getSpec, ok := resolvePath[pathToFile]
		if !ok {
			err1 := fmt.Errorf("path not found: %s", pathToFile)
			return nil, err1
		}
		return getSpec()
	}
	var specData []byte
	specData, err = rawSpec()
	if err != nil {
		return
	}
	swagger, err = loader.LoadFromData(specData)
	if err != nil {
		return
	}
	return
}
