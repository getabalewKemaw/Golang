package docs

import _ "embed"

// OpenAPIJSON contains the raw embedded OpenAPI 3.0.3 specification.
//
//go:embed openapi.json
var OpenAPIJSON []byte
