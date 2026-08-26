package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"task261-apimigproof/internal/model"
)

// sampleRow 是 samples 表的一行。
type sampleRow struct {
	id          int64
	fingerprint string
	payloadJSON string
	status      string
	importedAt  string
}

func scanSample(row interface{ Scan(...interface{}) error }) (model.RequestSample, error) {
	var r sampleRow
	if err := row.Scan(&r.id, &r.fingerprint, &r.payloadJSON, &r.status, &r.importedAt); err != nil {
		return model.RequestSample{}, err
	}
	return model.RequestSample{
		ID: r.id, Fingerprint: r.fingerprint, PayloadJSON: r.payloadJSON,
		Status: r.status, ImportedAt: mustParseTime(r.importedAt),
	}, nil
}

// ImportSample 导入请求样本。以内容指纹做幂等：重复导入返回已存在样本且 added=false。
func (s *Store) ImportSample(payload string) (model.RequestSample, bool, error) {
	fp, err := model.SampleFingerprint(payload)
	if err != nil {
		return model.RequestSample{}, false, err
	}
	// 先查已存在。
	var id int64
	var status string
	err = s.db.QueryRow(`SELECT id, status FROM samples WHERE fingerprint = ?`, fp).Scan(&id, &status)
	if err == nil {
		sm, gerr := s.GetSample(id)
		return sm, false, gerr
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return model.RequestSample{}, false, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`INSERT INTO samples (fingerprint, payload_json, status, imported_at) VALUES (?,?,?,?)`,
		fp, payload, model.SampleOriginal, now)
	if err != nil {
		// 并发插入冲突：以查询结果为准。
		if err2 := s.db.QueryRow(`SELECT id FROM samples WHERE fingerprint = ?`, fp).Scan(&id); err2 == nil {
			sm, gerr := s.GetSample(id)
			return sm, false, gerr
		}
		return model.RequestSample{}, false, err
	}
	nid, _ := res.LastInsertId()
	sm, gerr := s.GetSample(nid)
	return sm, true, gerr
}

// GetSample 按 ID 读取样本。
func (s *Store) GetSample(id int64) (model.RequestSample, error) {
	row := s.db.QueryRow(`SELECT id, fingerprint, payload_json, status, imported_at FROM samples WHERE id = ?`, id)
	sm, err := scanSample(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.RequestSample{}, fmt.Errorf("%w: sample %d", model.ErrNotFound, id)
	}
	return sm, err
}

// ListSamples 列出全部样本（按导入时间倒序）。
func (s *Store) ListSamples() ([]model.RequestSample, error) {
	rows, err := s.db.Query(`SELECT id, fingerprint, payload_json, status, imported_at FROM samples ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.RequestSample, 0)
	for rows.Next() {
		sm, err := scanSample(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sm)
	}
	return out, rows.Err()
}

// SetSampleStatus 更新样本状态（比较后回写）。
func (s *Store) SetSampleStatus(id int64, status string) error {
	_, err := s.db.Exec(`UPDATE samples SET status = ? WHERE id = ?`, status, id)
	return err
}

// DeleteSample 删除样本。若样本被已发布证明引用则拒绝。
func (s *Store) DeleteSample(id int64) error {
	var refs int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM comparison_results cr
		JOIN proofs p ON p.comparison_id = cr.comparison_id AND p.status != ?
		WHERE cr.sample_id = ?`, model.ProofDraft, id).Scan(&refs)
	if err != nil {
		return err
	}
	if refs > 0 {
		return fmt.Errorf("%w: sample %d", model.ErrSampleReferenced, id)
	}
	res, err := s.db.Exec(`DELETE FROM samples WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: sample %d", model.ErrNotFound, id)
	}
	return nil
}

// CountSamples 统计样本总数。
func (s *Store) CountSamples() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM samples`).Scan(&n)
	return n, err
}
