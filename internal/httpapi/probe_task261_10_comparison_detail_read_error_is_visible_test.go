package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"task261-apimigproof/internal/service"
	"task261-apimigproof/internal/store"
)

func TestBug10ComparisonDetailReadErrorIsVisible(t *testing.T) {
	st, err := store.OpenStore(filepath.Join(t.TempDir(), "broken.db")); if err != nil { t.Fatal(err) }
	defer st.Close()
	from, err := st.CreateContract("orders", 1); if err != nil { t.Fatal(err) }
	to, err := st.CreateContract("orders", 2); if err != nil { t.Fatal(err) }
	c, err := st.CreateComparison(from.ID, to.ID, nil); if err != nil { t.Fatal(err) }
	if _, err := st.DB().Exec("DROP TABLE comparison_results"); err != nil { t.Fatal(err) }
	req := httptest.NewRequest(http.MethodGet, "/api/comparisons/"+itoa(c.ID), nil)
	rec := httptest.NewRecorder()
	New(service.New(st)).Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError { t.Fatalf("status=%d body=%s, want 500", rec.Code, rec.Body.String()) }
}

func itoa(v int64) string { return fmt.Sprintf("%d", v) }
