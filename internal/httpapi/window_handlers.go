package httpapi

import (
	"net/http"

	"task261-apimigproof/internal/model"
)

// windowReq 兼容窗口请求。
type windowReq struct {
	FromContractID int64   `json:"from_contract_id"`
	ToContractID   int64   `json:"to_contract_id"`
	Policy         string  `json:"policy"`
	Note           string  `json:"note"`
	ValidUntil     *string `json:"valid_until,omitempty"`
}

// handleCreateWindow POST /api/windows
func (s *Server) handleCreateWindow(w http.ResponseWriter, r *http.Request) {
	var req windowReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid request body: "+err.Error()))
		return
	}
	if req.Policy == "" {
		req.Policy = model.PolicyTransform
	}
	win, err := s.svc.Window.Declare(req.FromContractID, req.ToContractID,
		req.Policy, req.Note, req.ValidUntil)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, win)
}

// handleListWindows GET /api/windows
func (s *Server) handleListWindows(w http.ResponseWriter, _ *http.Request) {
	list, err := s.svc.Window.List()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"windows": list, "count": len(list)})
}

// windowUpdateReq 更新窗口策略请求。
type windowUpdateReq struct {
	Policy string `json:"policy"`
}

// handleUpdateWindow PUT /api/windows/{id}
func (s *Server) handleUpdateWindow(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid window id"))
		return
	}
	var req windowUpdateReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid request body: "+err.Error()))
		return
	}
	win, err := s.svc.Window.UpdatePolicy(id, req.Policy)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, win)
}
