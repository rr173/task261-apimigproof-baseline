package httpapi

import (
	"net/http"

	"task261-apimigproof/internal/model"
)

// sampleReq 单个样本导入请求。
type sampleReq struct {
	Payload string `json:"payload"`
}

// handleImportSample POST /api/samples
func (s *Server) handleImportSample(w http.ResponseWriter, r *http.Request) {
	var req sampleReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid request body: "+err.Error()))
		return
	}
	if req.Payload == "" {
		writeErr(w, model.NewStatusError(400, "bad_request", "payload is required"))
		return
	}
	sm, added, err := s.svc.Store.ImportSample(req.Payload)
	if err != nil {
		writeErr(w, err)
		return
	}
	status := http.StatusCreated
	if !added {
		status = http.StatusOK // 幂等命中：返回已存在样本
	}
	writeJSON(w, status, map[string]any{"sample": sm, "added": added})
}

// batchReq 批量导入请求。
type batchReq struct {
	Payloads []string `json:"payloads"`
}

// handleImportBatch POST /api/samples/batch
// 样本可并行导入（此处串行处理，幂等指纹保证同一请求只保留一条）。
func (s *Server) handleImportBatch(w http.ResponseWriter, r *http.Request) {
	var req batchReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid request body: "+err.Error()))
		return
	}
	if len(req.Payloads) == 0 {
		writeErr(w, model.NewStatusError(400, "bad_request", "payloads must not be empty"))
		return
	}
	type batchItem struct {
		Fingerprint string `json:"fingerprint"`
		ID          int64  `json:"id"`
		Added       bool   `json:"added"`
	}
	out := make([]batchItem, 0, len(req.Payloads))
	addedCount := 0
	for _, pl := range req.Payloads {
		if pl == "" {
			continue
		}
		sm, added, err := s.svc.Store.ImportSample(pl)
		if err != nil {
			writeErr(w, err)
			return
		}
		if added {
			addedCount++
		}
		out = append(out, batchItem{Fingerprint: sm.Fingerprint, ID: sm.ID, Added: added})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out, "total": len(out), "added": addedCount,
	})
}

// handleListSamples GET /api/samples
func (s *Server) handleListSamples(w http.ResponseWriter, _ *http.Request) {
	list, err := s.svc.Store.ListSamples()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"samples": list, "count": len(list)})
}

// handleGetSample GET /api/samples/{id}
func (s *Server) handleGetSample(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid sample id"))
		return
	}
	sm, err := s.svc.Store.GetSample(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sm)
}

// handleDeleteSample DELETE /api/samples/{id}
func (s *Server) handleDeleteSample(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid sample id"))
		return
	}
	if err := s.svc.Store.DeleteSample(id); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
