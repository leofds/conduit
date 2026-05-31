// Package api embeds the OpenAPI specification for the Conduit API resolver contract.
package api

import _ "embed"

// Spec contains the raw bytes of openapi.yaml, bundled at compile time.
//
//go:embed openapi.yaml
var Spec []byte
