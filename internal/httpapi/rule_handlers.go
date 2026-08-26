package httpapi

import (
	"net/http"

	"task261-apimigproof/internal/model"
)

// ruleReq 转换规则请求。
type ruleReq struct {
	FromField   string  `json:"from_field"`
	ToField     string  `json:"to_field"`
	Action      string  `json:"action"`
	CoerceFrom  string  `json:"coerce_from"`
	CoerceTo    string  `json:"coerce_to"`
	DefaultJSON *string `json:"default_json,omitempty"`
	Precedence  int     `json:"precedence"`
}

// handleCreateRule POST /api/contracts/{id}/rules
func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid contract id"))
		return
	}
	var req ruleReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid request body: "+err.Error()))
		return
	}
	rule, err := s.svc.Store.CreateRule(model.TransformationRule{
		ContractID: id, FromField: req.FromField, ToField: req.ToField,
		Action: req.Action, CoerceFrom: req.CoerceFrom, CoerceTo: req.CoerceTo,
		DefaultJSON: req.DefaultJSON, Precedence: req.Precedence,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

// handleListRules GET /api/contracts/{id}/rules
func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid contract id"))
		return
	}
	rules, err := s.svc.Store.ListRules(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules, "count": len(rules)})
}

// handleDeleteRule DELETE /api/contracts/{id}/rules/{rid}
func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	rid, err := pathID(r, "rid")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid rule id"))
		return
	}
	if err := s.svc.Store.DeleteRule(rid); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
