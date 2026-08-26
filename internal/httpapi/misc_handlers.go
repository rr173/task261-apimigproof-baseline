package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"task261-apimigproof/internal/service"
	"task261-apimigproof/internal/store"
)

// handleHealth GET /api/health
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	var sampleCount int
	if n, err := s.svc.Store.CountSamples(); err == nil {
		sampleCount = n
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
		"samples": sampleCount,
	})
}

// handleSelfCheck POST /api/selfcheck
// 复用 --smoke-test 的端到端场景，在隔离数据库上执行并返回结果。
func (s *Server) handleSelfCheck(w http.ResponseWriter, _ *http.Request) {
	dir, err := os.MkdirTemp("", "apimigproof-selfcheck-*")
	if err != nil {
		writeErr(w, err)
		return
	}
	defer os.RemoveAll(dir)
	dbPath := filepath.Join(dir, "selfcheck.db")
	st, err := store.OpenStore(dbPath)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer st.Close()
	if _, _, err := service.New(st).SelfCheck(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "passed", "scenario": "migration-proof-e2e"})
}
