// Package proof 管理兼容窗口与迁移证明的发布。
//
// 迁移证明是迁移的最终产物：发布后绑定新旧契约版本、比较任务与样本证据
// 指纹，形成不可变快照；只允许用更新的证明替代（supersede），不可修改。
package proof

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"task261-apimigproof/internal/model"
)

// Publisher 是迁移证明的服务对象。
type Publisher struct {
	store Store
}

// Store 定义 Publisher 依赖的存储接口。
type Store interface {
	CreateProof(fromID, toID, comparisonID int64, summary, evidence string) (model.MigrationProof, error)
	GetProof(id int64) (model.MigrationProof, error)
	ListProofs() ([]model.MigrationProof, error)
	PublishProof(id int64) (model.MigrationProof, error)
	SupersedeProof(oldID, newID int64) (model.MigrationProof, error)
	GetComparison(id int64) (model.Comparison, error)
}

// NewPublisher 构造证明发布器。
func NewPublisher(s Store) *Publisher { return &Publisher{store: s} }

// Create 基于一次已完成的比较创建迁移证明草稿。
func (p *Publisher) Create(comparisonID int64) (model.MigrationProof, error) {
	c, err := p.store.GetComparison(comparisonID)
	if err != nil {
		return model.MigrationProof{}, err
	}
	if c.Status == model.ComparisonRunning {
		return model.MigrationProof{}, fmt.Errorf("%w: comparison %d has not finished", model.ErrBadRequest, comparisonID)
	}
	if c.Status == model.ComparisonFailed {
		return model.MigrationProof{}, fmt.Errorf("%w: comparison %d failed", model.ErrBadRequest, comparisonID)
	}
	if c.Status != model.ComparisonCompatible || c.TotalSamples == 0 ||
		c.SemanticsChanged != 0 || c.Rejected != 0 {
		return model.MigrationProof{}, fmt.Errorf("%w: comparison %d is not a non-empty compatible result", model.ErrBadRequest, comparisonID)
	}
	summary, err := buildSummary(c)
	if err != nil {
		return model.MigrationProof{}, err
	}
	evidence, err := buildEvidenceFingerprint(c.Results)
	if err != nil {
		return model.MigrationProof{}, err
	}
	return p.store.CreateProof(c.FromContractID, c.ToContractID, c.ID, summary, evidence)
}

// Publish 发布证明（封存为不可变）。
func (p *Publisher) Publish(id int64) (model.MigrationProof, error) {
	return p.store.PublishProof(id)
}

// Supersede 用新证明替代旧证明。
func (p *Publisher) Supersede(oldID, newID int64) (model.MigrationProof, error) {
	return p.store.SupersedeProof(oldID, newID)
}

// Get 读取证明详情。
func (p *Publisher) Get(id int64) (model.MigrationProof, error) {
	return p.store.GetProof(id)
}

// List 列出全部证明。
func (p *Publisher) List() ([]model.MigrationProof, error) {
	return p.store.ListProofs()
}

// buildSummary 汇总比较统计为 JSON 摘要。
func buildSummary(c model.Comparison) (string, error) {
	s := map[string]any{
		"from_contract_id":  c.FromContractID,
		"to_contract_id":    c.ToContractID,
		"status":            c.Status,
		"total_samples":     c.TotalSamples,
		"migratable":        c.Migratable,
		"rejected":          c.Rejected,
		"semantics_changed": c.SemanticsChanged,
		"window_id":         c.WindowID,
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// buildEvidenceFingerprint 基于样本判定明细构造证据指纹。
// 指纹覆盖（样本指纹, 判定, 问题类别）三元组，证明封存时的证据集合。
func buildEvidenceFingerprint(results []model.SampleResult) (string, error) {
	type triple struct {
		FP      string `json:"fp"`
		Verdict string `json:"verdict"`
		Issue   string `json:"issue"`
	}
	triples := make([]triple, 0, len(results))
	for _, r := range results {
		triples = append(triples, triple{FP: r.Fingerprint, Verdict: r.Verdict, Issue: r.Issue})
	}
	sort.Slice(triples, func(i, j int) bool { return triples[i].FP < triples[j].FP })
	b, err := json.Marshal(triples)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
