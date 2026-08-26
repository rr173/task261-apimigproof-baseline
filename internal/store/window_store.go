package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"task261-apimigproof/internal/model"
)

// windowRow 是 windows 表的一行。
type windowRow struct {
	id             int64
	fromContractID int64
	toContractID   int64
	policy         string
	validUntil     sql.NullString
	note           string
	createdAt      string
}

func scanWindow(row interface{ Scan(...interface{}) error }) (model.CompatWindow, error) {
	var r windowRow
	if err := row.Scan(&r.id, &r.fromContractID, &r.toContractID, &r.policy,
		&r.validUntil, &r.note, &r.createdAt); err != nil {
		return model.CompatWindow{}, err
	}
	w := model.CompatWindow{
		ID: r.id, FromContractID: r.fromContractID, ToContractID: r.toContractID,
		Policy: r.policy, Note: r.note, CreatedAt: mustParseTime(r.createdAt),
	}
	if r.validUntil.Valid {
		v := r.validUntil.String
		w.ValidUntil = &v
	}
	return w, nil
}

// CreateWindow 声明兼容窗口。
func (s *Store) CreateWindow(w model.CompatWindow) (model.CompatWindow, error) {
	if err := model.ValidatePolicy(w.Policy); err != nil {
		return model.CompatWindow{}, err
	}
	var until any
	if w.ValidUntil != nil {
		until = *w.ValidUntil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`
		INSERT INTO windows (from_contract_id, to_contract_id, policy, valid_until, note, created_at)
		VALUES (?,?,?,?,?,?)`,
		w.FromContractID, w.ToContractID, w.Policy, until, w.Note, now)
	if err != nil {
		return model.CompatWindow{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetWindow(id)
}

// GetWindow 按 ID 读取兼容窗口。
func (s *Store) GetWindow(id int64) (model.CompatWindow, error) {
	row := s.db.QueryRow(`
		SELECT id, from_contract_id, to_contract_id, policy, valid_until, note, created_at
		FROM windows WHERE id = ?`, id)
	w, err := scanWindow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.CompatWindow{}, fmt.Errorf("%w: window %d", model.ErrNotFound, id)
	}
	return w, err
}

// ListWindows 列出全部兼容窗口。
func (s *Store) ListWindows() ([]model.CompatWindow, error) {
	rows, err := s.db.Query(`
		SELECT id, from_contract_id, to_contract_id, policy, valid_until, note, created_at
		FROM windows ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.CompatWindow, 0)
	for rows.Next() {
		w, err := scanWindow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// UpdateWindowPolicy 更新兼容窗口的拒绝策略。
func (s *Store) UpdateWindowPolicy(id int64, policy string) error {
	if err := model.ValidatePolicy(policy); err != nil {
		return err
	}
	res, err := s.db.Exec(`UPDATE windows SET policy = ? WHERE id = ?`, policy, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: window %d", model.ErrNotFound, id)
	}
	return nil
}

// FindWindow 查找绑定同一契约对的最新窗口。
func (s *Store) FindWindow(fromID, toID int64) (*model.CompatWindow, error) {
	row := s.db.QueryRow(`
		SELECT id, from_contract_id, to_contract_id, policy, valid_until, note, created_at
		FROM windows WHERE from_contract_id = ? AND to_contract_id = ?
		ORDER BY id DESC LIMIT 1`, fromID, toID)
	w, err := scanWindow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}
