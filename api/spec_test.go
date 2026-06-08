package api

import (
	"strings"
	"testing"
)

func TestSpecContainsOpenAPIDocument(t *testing.T) {
	if len(Spec) == 0 {
		t.Fatal("Spec is empty")
	}

	if !strings.Contains(string(Spec), "openapi:") {
		t.Fatal("Spec does not contain an OpenAPI document")
	}
}
