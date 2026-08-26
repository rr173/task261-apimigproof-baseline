package httpapi

import (
	"net/http"

	"task261-apimigproof/internal/model"
)

// proofReq 创建迁移证明请求。
type proofReq struct {
	ComparisonID int64 `json:"comparison_id"`
}

// handleCreateProof POST /api/proofs
func (s *Server) handleCreateProof(w http.ResponseWriter, r *http.Request) {
	var req proofReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid request body: "+err.Error()))
		return
	}
	if req.ComparisonID <= 0 {
		writeErr(w, model.NewStatusError(400, "bad_request", "comparison_id is required"))
		return
	}
	p, err := s.svc.Proofs.Create(req.ComparisonID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// handleListProofs GET /api/proofs
func (s *Server) handleListProofs(w http.ResponseWriter, _ *http.Request) {
	list, err := s.svc.Proofs.List()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"proofs": list, "count": len(list)})
}

// handleGetProof GET /api/proofs/{id}
func (s *Server) handleGetProof(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid proof id"))
		return
	}
	p, err := s.svc.Proofs.Get(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// handlePublishProof POST /api/proofs/{id}/publish
func (s *Server) handlePublishProof(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid proof id"))
		return
	}
	p, err := s.svc.Proofs.Publish(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// supersedeReq 替代请求：声明新证明 ID。
type supersedeReq struct {
	NewProofID int64 `json:"new_proof_id"`
}

// handleSupersedeProof POST /api/proofs/{id}/supersede
// 用新证明替代旧证明：旧证明标记 superseded 并绑定新证明。
func (s *Server) handleSupersedeProof(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid proof id"))
		return
	}
	var req supersedeReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid request body: "+err.Error()))
		return
	}
	if req.NewProofID <= 0 || req.NewProofID == id {
		writeErr(w, model.NewStatusError(400, "bad_request", "new_proof_id must be a different positive id"))
		return
	}
	p, err := s.svc.Proofs.Supersede(id, req.NewProofID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
