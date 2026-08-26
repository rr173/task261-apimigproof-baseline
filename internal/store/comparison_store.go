package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"task261-apimigproof/internal/model"
)

// comparisonRow 是 comparisons 表的一行。
type comparisonRow struct {
	id               int64
	fromContractID   int64
	toContractID     int64
	status           string
	totalSamples     int
	migratable       int
	rejected         int
	semanticsChanged int
	windowID         sql.NullInt64
	createdAt        string
	finishedAt       sql.NullString
}

func scanComparison(row interface{ Scan(...interface{}) error }) (model.Comparison, error) {
	var r comparisonRow
	if err := row.Scan(&r.id, &r.fromContractID, &r.toContractID, &r.status,
		&r.totalSamples, &r.migratable, &r.rejected, &r.semanticsChanged,
		&r.windowID, &r.createdAt, &r.finishedAt); err != nil {
		return model.Comparison{}, err
	}
	c := model.Comparison{
		ID: r.id, FromContractID: r.fromContractID, ToContractID: r.toContractID,
		Status: r.status, TotalSamples: r.totalSamples, Migratable: r.migratable,
		Rejected: r.rejected, SemanticsChanged: r.semanticsChanged,
		CreatedAt: mustParseTime(r.createdAt),
	}
	if r.windowID.Valid {
		v := r.windowID.Int64
		c.WindowID = &v
	}
	if r.finishedAt.Valid {
		t := mustParseTime(r.finishedAt.String)
		c.FinishedAt = &t
	}
	return c, nil
}

// CreateComparison 创建比较任务。同一契约对不允许同时存在 running 任务。
func (s *Store) CreateComparison(fromID, toID int64, windowID *int64) (model.Comparison, error) {
	var running int
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM comparisons WHERE from_contract_id = ? AND to_contract_id = ? AND status = ?`,
		fromID, toID, model.ComparisonRunning).Scan(&running); err != nil {
		return model.Comparison{}, err
	}
	if running > 0 {
		return model.Comparison{}, fmt.Errorf("%w: contracts %d -> %d", model.ErrCompareRunning, fromID, toID)
	}
	var wid any
	if windowID != nil {
		wid = *windowID
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`
		INSERT INTO comparisons (from_contract_id, to_contract_id, status,
			total_samples, migratable, rejected, semantics_changed, window_id, created_at, finished_at)
		VALUES (?,?,?,0,0,0,0,?,?,NULL)`,
		fromID, toID, model.ComparisonRunning, wid, now)
	if err != nil {
		return model.Comparison{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetComparison(id)
}

// GetComparison 按 ID 读取比较任务（含明细）。
func (s *Store) GetComparison(id int64) (model.Comparison, error) {
	row := s.db.QueryRow(`
		SELECT id, from_contract_id, to_contract_id, status,
			total_samples, migratable, rejected, semantics_changed, window_id, created_at, finished_at
		FROM comparisons WHERE id = ?`, id)
	c, err := scanComparison(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Comparison{}, fmt.Errorf("%w: comparison %d", model.ErrNotFound, id)
	}
	if err != nil {
		return model.Comparison{}, err
	}
	c.Results, err = s.ListComparisonResults(id)
	if err != nil {
		return model.Comparison{}, err
	}
	c.Issues, err = s.ListComparisonIssues(id)
	if err != nil {
		return model.Comparison{}, err
	}
	return c, nil
}

// ListComparisons 列出全部比较任务。
func (s *Store) ListComparisons() ([]model.Comparison, error) {
	rows, err := s.db.Query(`
		SELECT id, from_contract_id, to_contract_id, status,
			total_samples, migratable, rejected, semantics_changed, window_id, created_at, finished_at
		FROM comparisons ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Comparison, 0)
	for rows.Next() {
		c, err := scanComparison(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// FinishComparison 写回比较结果与终态。
func (s *Store) FinishComparison(c model.Comparison, results []model.SampleResult, issues []model.Issue) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec(`
		UPDATE comparisons SET status = ?, total_samples = ?, migratable = ?, rejected = ?,
			semantics_changed = ?, finished_at = ? WHERE id = ?`,
		c.Status, c.TotalSamples, c.Migratable, c.Rejected, c.SemanticsChanged, now, c.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM comparison_results WHERE comparison_id = ?`, c.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM comparison_issues WHERE comparison_id = ?`, c.ID); err != nil {
		return err
	}
	for _, r := range results {
		if _, err := tx.Exec(`
			INSERT INTO comparison_results (comparison_id, sample_id, verdict, issue, detail)
			VALUES (?,?,?,?,?)`, c.ID, r.SampleID, r.Verdict, r.Issue, r.Detail); err != nil {
			return err
		}
	}
	for _, is := range issues {
		if _, err := tx.Exec(`
			INSERT INTO comparison_issues (comparison_id, sample_id, kind, field, message)
			VALUES (?,?,?,?,?)`, c.ID, is.SampleID, is.Kind, is.Field, is.Message); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListComparisonResults 读取比较任务的样本判定明细。
func (s *Store) ListComparisonResults(comparisonID int64) ([]model.SampleResult, error) {
	rows, err := s.db.Query(`
		SELECT cr.sample_id, sm.fingerprint, cr.verdict, cr.issue, cr.detail
		FROM comparison_results cr JOIN samples sm ON sm.id = cr.sample_id
		WHERE cr.comparison_id = ? ORDER BY cr.sample_id`, comparisonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.SampleResult, 0)
	for rows.Next() {
		var r model.SampleResult
		if err := rows.Scan(&r.SampleID, &r.Fingerprint, &r.Verdict, &r.Issue, &r.Detail); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListComparisonIssues 读取比较任务发现的问题清单。
func (s *Store) ListComparisonIssues(comparisonID int64) ([]model.Issue, error) {
	rows, err := s.db.Query(`
		SELECT sample_id, kind, field, message FROM comparison_issues
		WHERE comparison_id = ? ORDER BY id`, comparisonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Issue, 0)
	for rows.Next() {
		var is model.Issue
		if err := rows.Scan(&is.SampleID, &is.Kind, &is.Field, &is.Message); err != nil {
			return nil, err
		}
		out = append(out, is)
	}
	return out, rows.Err()
}
