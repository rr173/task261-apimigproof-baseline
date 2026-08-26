package httpapi

import (
	"net/http"
	"strconv"

	"task261-apimigproof/internal/model"
)

// contractReq 创建契约版本请求。
type contractReq struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// handleCreateContract POST /api/contracts
func (s *Server) handleCreateContract(w http.ResponseWriter, r *http.Request) {
	var req contractReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid request body: "+err.Error()))
		return
	}
	if req.Name == "" || req.Version <= 0 {
		writeErr(w, model.NewStatusError(400, "bad_request", "name and positive version are required"))
		return
	}
	c, err := s.svc.Reg.CreateContract(req.Name, req.Version)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// handleListContracts GET /api/contracts
func (s *Server) handleListContracts(w http.ResponseWriter, _ *http.Request) {
	list, err := s.svc.Store.ListContracts()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"contracts": list, "count": len(list)})
}

// handleGetContract GET /api/contracts/{id}
func (s *Server) handleGetContract(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid contract id"))
		return
	}
	c, err := s.svc.Reg.GetContract(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	fields, _ := s.svc.Store.ListFields(id)
	writeJSON(w, http.StatusOK, map[string]any{"contract": c, "fields": fields})
}

// fieldReq 字段语义请求。
type fieldReq struct {
	FieldID      string `json:"field_id"`
	Status       string `json:"status"`
	ValueType    string `json:"value_type"`
	HasDefault   bool   `json:"has_default"`
	DefaultValue *string `json:"default_value,omitempty"`
	Description  string `json:"description"`
}

// handleAddField POST /api/contracts/{id}/fields
func (s *Server) handleAddField(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid contract id"))
		return
	}
	var req fieldReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid request body: "+err.Error()))
		return
	}
	f, err := s.svc.Reg.AddField(model.FieldSemantics{
		ContractID: id, FieldID: req.FieldID, Status: req.Status,
		ValueType: req.ValueType, HasDefault: req.HasDefault,
		DefaultValue: req.DefaultValue, Description: req.Description,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

// handleUpdateField PUT /api/contracts/{id}/fields/{field}
func (s *Server) handleUpdateField(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid contract id"))
		return
	}
	fieldID := r.PathValue("field")
	var req fieldReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid request body: "+err.Error()))
		return
	}
	f, err := s.svc.Reg.UpdateField(model.FieldSemantics{
		ContractID: id, FieldID: fieldID, Status: req.Status,
		ValueType: req.ValueType, HasDefault: req.HasDefault,
		DefaultValue: req.DefaultValue, Description: req.Description,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// handleRemoveField DELETE /api/contracts/{id}/fields/{field}
func (s *Server) handleRemoveField(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid contract id"))
		return
	}
	if err := s.svc.Reg.RemoveField(id, r.PathValue("field")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// handleListFields GET /api/contracts/{id}/fields
func (s *Server) handleListFields(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid contract id"))
		return
	}
	fields, err := s.svc.Store.ListFields(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"fields": fields, "count": len(fields)})
}

// handleSealContract POST /api/contracts/{id}/seal
func (s *Server) handleSealContract(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.NewStatusError(400, "bad_request", "invalid contract id"))
		return
	}
	c, err := s.svc.Reg.Seal(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// parseFieldStatus 校验字段状态；空串视为 valid。
func parseFieldStatus(st string) string {
	if st == "" {
		return model.FieldValid
	}
	return st
}

// intString 便捷转换。
func intString(v int64) string { return strconv.FormatInt(v, 10) }
