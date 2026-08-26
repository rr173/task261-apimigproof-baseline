package httpapi

import (
	"net/http"

	"task261-apimigproof/internal/model"
)

// compareReq 触发比较请求。
type compareReq struct {
	FromContractID int64 `json:"from_contract_id"`
	ToContractID   int64 `json:"to_contract_id"`
}

// handleRunComparison POST /api/compare
func (s *Server) handleRunComparison(w http.ResponseWriter, r *http.Request) {
	var req compareReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid request body: "+err.Error()))
		return
	}
	if req.FromContractID <= 0 || req.ToContractID <= 0 || req.FromContractID == req.ToContractID {
		writeErr(w, model.NewStatusError(400, "bad_request",
			"from_contract_id and to_contract_id must be distinct positive ids"))
		return
	}
	comp, err := s.svc.RunComparison(req.FromContractID, req.ToContractID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, comp)
}

// handleListComparisons GET /api/comparisons
func (s *Server) handleListComparisons(w http.ResponseWriter, _ *http.Request) {
	list, err := s.svc.Store.ListComparisons()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"comparisons": list, "count": len(list)})
}

// handleGetComparison GET /api/comparisons/{id}
func (s *Server) handleGetComparison(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid comparison id"))
		return
	}
	comp, err := s.svc.Store.GetComparison(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, comp)
}

// handleComparisonIssues GET /api/comparisons/{id}/issues
func (s *Server) handleComparisonIssues(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid comparison id"))
		return
	}
	issues, err := s.svc.Store.ListComparisonIssues(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"issues": issues, "count": len(issues)})
}
