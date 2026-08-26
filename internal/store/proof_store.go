package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"task261-apimigproof/internal/model"
)

// proofRow 是 proofs 表的一行。
type proofRow struct {
	id                 int64
	fromContractID     int64
	toContractID       int64
	comparisonID       int64
	status             string
	summaryJSON        string
	evidenceFingerprint string
	createdAt          string
	publishedAt        sql.NullString
	supersededBy       sql.NullInt64
}

func scanProof(row interface{ Scan(...interface{}) error }) (model.MigrationProof, error) {
	var r proofRow
	if err := row.Scan(&r.id, &r.fromContractID, &r.toContractID, &r.comparisonID,
		&r.status, &r.summaryJSON, &r.evidenceFingerprint, &r.createdAt,
		&r.publishedAt, &r.supersededBy); err != nil {
		return model.MigrationProof{}, err
	}
	p := model.MigrationProof{
		ID: r.id, FromContractID: r.fromContractID, ToContractID: r.toContractID,
		ComparisonID: r.comparisonID, Status: r.status,
		SummaryJSON: r.summaryJSON, EvidenceFingerprint: r.evidenceFingerprint,
		CreatedAt: mustParseTime(r.createdAt),
	}
	if r.publishedAt.Valid {
		t := mustParseTime(r.publishedAt.String)
		p.PublishedAt = &t
	}
	if r.supersededBy.Valid {
		v := r.supersededBy.Int64
		p.SupersededBy = &v
	}
	return p, nil
}

// CreateProof 创建迁移证明（草稿态）。
func (s *Store) CreateProof(fromID, toID, comparisonID int64, summary, evidence string) (model.MigrationProof, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`
		INSERT INTO proofs (from_contract_id, to_contract_id, comparison_id, status,
			summary_json, evidence_fingerprint, created_at, published_at, superseded_by)
		VALUES (?,?,?,?,?,?,?,NULL,NULL)`,
		fromID, toID, comparisonID, model.ProofDraft, summary, evidence, now)
	if err != nil {
		return model.MigrationProof{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetProof(id)
}

// GetProof 按 ID 读取证明。
func (s *Store) GetProof(id int64) (model.MigrationProof, error) {
	row := s.db.QueryRow(`
		SELECT id, from_contract_id, to_contract_id, comparison_id, status,
			summary_json, evidence_fingerprint, created_at, published_at, superseded_by
		FROM proofs WHERE id = ?`, id)
	p, err := scanProof(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.MigrationProof{}, fmt.Errorf("%w: proof %d", model.ErrNotFound, id)
	}
	return p, err
}

// ListProofs 列出全部迁移证明。
func (s *Store) ListProofs() ([]model.MigrationProof, error) {
	rows, err := s.db.Query(`
		SELECT id, from_contract_id, to_contract_id, comparison_id, status,
			summary_json, evidence_fingerprint, created_at, published_at, superseded_by
		FROM proofs ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.MigrationProof, 0)
	for rows.Next() {
		p, err := scanProof(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PublishProof 发布证明（封存）。仅允许 draft → published。
func (s *Store) PublishProof(id int64) (model.MigrationProof, error) {
	p, err := s.GetProof(id)
	if err != nil {
		return model.MigrationProof{}, err
	}
	if p.Status != model.ProofDraft {
		return model.MigrationProof{}, fmt.Errorf("%w: proof %d is %s", model.ErrProofSealed, id, p.Status)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`
		UPDATE proofs SET status = ?, published_at = ? WHERE id = ?`,
		model.ProofPublished, now, id); err != nil {
		return model.MigrationProof{}, err
	}
	return s.GetProof(id)
}

// SupersedeProof 用新证明替代旧证明：旧证明标记 superseded 并指向新证明。
func (s *Store) SupersedeProof(oldID, newID int64) (model.MigrationProof, error) {
	p, err := s.GetProof(oldID)
	if err != nil {
		return model.MigrationProof{}, err
	}
	if p.Status != model.ProofPublished {
		return model.MigrationProof{}, fmt.Errorf("%w: proof %d is not published", model.ErrProofSealed, oldID)
	}
	if _, err := s.db.Exec(`
		UPDATE proofs SET status = ?, superseded_by = ? WHERE id = ?`,
		model.ProofSuperseded, newID, oldID); err != nil {
		return model.MigrationProof{}, err
	}
	return s.GetProof(oldID)
}

// CountProofsForComparison 统计已绑定某比较任务的证明数量。
func (s *Store) CountProofsForComparison(comparisonID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM proofs WHERE comparison_id = ?`, comparisonID).Scan(&n)
	return n, err
}
