package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"task261-apimigproof/internal/service"
	"task261-apimigproof/internal/store"
)

func TestHealthEndpointReportsOK(t *testing.T) {
	st, err := store.OpenStore(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer st.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	New(service.New(st)).Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
}

func TestCreateContractRejectsUnknownJSONField(t *testing.T) {
	st, err := store.OpenStore(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer st.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/contracts", stringsReader(`{"name":"orders","version":1,"extra":true}`))
	rec := httptest.NewRecorder()
	New(service.New(st)).Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func stringsReader(s string) *strings.Reader { return strings.NewReader(s) }
