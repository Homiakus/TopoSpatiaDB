package topospatia

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerDocsAndOpenAPI(t *testing.T) {
	opts := Options{
		DataDir:  t.TempDir(),
		MaxDim:   2,
		StalkDim: 2,
		Fsync:    false,
	}
	db, err := Open(opts)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	srv := NewServer(db.Sheaf(), db.Storage(), db.Consensus())

	// Test OpenAPI spec
	req := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for openapi.json, got %d", w.Code)
	}
	var spec map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatalf("Failed to parse OpenAPI JSON: %v", err)
	}
	if spec["openapi"] != "3.0.3" {
		t.Errorf("Expected OpenAPI version 3.0.3, got %v", spec["openapi"])
	}

	// Test Swagger UI
	reqDocs := httptest.NewRequest(http.MethodGet, "/docs", nil)
	wDocs := httptest.NewRecorder()
	srv.ServeHTTP(wDocs, reqDocs)

	if wDocs.Code != http.StatusOK {
		t.Fatalf("Expected 200 for /docs, got %d", wDocs.Code)
	}
	if !strings.Contains(wDocs.Body.String(), "SwaggerUIBundle") {
		t.Errorf("Expected SwaggerUIBundle in /docs output")
	}
}
