// Package httpapi 提供 /api 前缀的 HTTP 接口层。
//
// 所有 handler 均为薄封装：解析请求 → 调用 service 用例 → 错误映射 →
// JSON 响应。错误经 model.Classify 映射为可读的 {code, message} 结构。
package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/service"
)

// Server 是 HTTP 服务对象。
type Server struct {
	svc *service.Service
	mux *http.ServeMux
}

// New 构造 HTTP 服务并注册全部路由。
func New(svc *service.Service) *Server {
	s := &Server{svc: svc, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler 返回可挂载到 http.Server 的处理器。
func (s *Server) Handler() http.Handler { return s.mux }

// routes 注册全部 /api 路由。
func (s *Server) routes() {
	// 契约与字段语义。
	s.mux.HandleFunc("POST /api/contracts", s.handleCreateContract)
	s.mux.HandleFunc("GET /api/contracts", s.handleListContracts)
	s.mux.HandleFunc("GET /api/contracts/{id}", s.handleGetContract)
	s.mux.HandleFunc("POST /api/contracts/{id}/fields", s.handleAddField)
	s.mux.HandleFunc("PUT /api/contracts/{id}/fields/{field}", s.handleUpdateField)
	s.mux.HandleFunc("DELETE /api/contracts/{id}/fields/{field}", s.handleRemoveField)
	s.mux.HandleFunc("GET /api/contracts/{id}/fields", s.handleListFields)
	s.mux.HandleFunc("POST /api/contracts/{id}/seal", s.handleSealContract)

	// 转换规则。
	s.mux.HandleFunc("POST /api/contracts/{id}/rules", s.handleCreateRule)
	s.mux.HandleFunc("GET /api/contracts/{id}/rules", s.handleListRules)
	s.mux.HandleFunc("DELETE /api/contracts/{id}/rules/{rid}", s.handleDeleteRule)

	// 请求样本。
	s.mux.HandleFunc("POST /api/samples", s.handleImportSample)
	s.mux.HandleFunc("POST /api/samples/batch", s.handleImportBatch)
	s.mux.HandleFunc("GET /api/samples", s.handleListSamples)
	s.mux.HandleFunc("GET /api/samples/{id}", s.handleGetSample)
	s.mux.HandleFunc("DELETE /api/samples/{id}", s.handleDeleteSample)

	// 兼容窗口。
	s.mux.HandleFunc("POST /api/windows", s.handleCreateWindow)
	s.mux.HandleFunc("GET /api/windows", s.handleListWindows)
	s.mux.HandleFunc("PUT /api/windows/{id}", s.handleUpdateWindow)

	// 比较。
	s.mux.HandleFunc("POST /api/compare", s.handleRunComparison)
	s.mux.HandleFunc("GET /api/comparisons", s.handleListComparisons)
	s.mux.HandleFunc("GET /api/comparisons/{id}", s.handleGetComparison)
	s.mux.HandleFunc("GET /api/comparisons/{id}/issues", s.handleComparisonIssues)

	// 迁移证明。
	s.mux.HandleFunc("POST /api/proofs", s.handleCreateProof)
	s.mux.HandleFunc("GET /api/proofs", s.handleListProofs)
	s.mux.HandleFunc("GET /api/proofs/{id}", s.handleGetProof)
	s.mux.HandleFunc("POST /api/proofs/{id}/publish", s.handlePublishProof)
	s.mux.HandleFunc("POST /api/proofs/{id}/supersede", s.handleSupersedeProof)

	// 杂项与自检。
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/selfcheck", s.handleSelfCheck)
}

// ---- 响应辅助 ----

// writeJSON 写 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

// errBody 是错误响应结构。
type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeErr 写错误响应（经 Classify 映射）。
func writeErr(w http.ResponseWriter, err error) {
	classified := model.Classify(err)
	if se, ok := classified.(*model.StatusError); ok {
		writeJSON(w, se.Status, errBody{Code: se.Code, Message: se.Msg})
		return
	}
	writeJSON(w, http.StatusInternalServerError, errBody{Code: "internal", Message: err.Error()})
}

// pathID 解析路径参数 {id} 为 int64。
func pathID(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(r.PathValue(key), 10, 64)
}

// decodeBody 解析 JSON 请求体到目标结构。
func decodeBody(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
