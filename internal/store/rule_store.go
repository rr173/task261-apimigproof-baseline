package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"task261-apimigproof/internal/model"
)

// ruleRow 是 rules 表的一行。
type ruleRow struct {
	id         int64
	contractID int64
	fromField  string
	toField    string
	action     string
	coerceFrom string
	coerceTo   string
	defaultVal sql.NullString
	precedence int
	createdAt  string
}

func scanRule(row interface{ Scan(...interface{}) error }) (model.TransformationRule, error) {
	var r ruleRow
	if err := row.Scan(&r.id, &r.contractID, &r.fromField, &r.toField, &r.action,
		&r.coerceFrom, &r.coerceTo, &r.defaultVal, &r.precedence, &r.createdAt); err != nil {
		return model.TransformationRule{}, err
	}
	rule := model.TransformationRule{
		ID: r.id, ContractID: r.contractID, FromField: r.fromField, ToField: r.toField,
		Action: r.action, CoerceFrom: r.coerceFrom, CoerceTo: r.coerceTo,
		Precedence: r.precedence, CreatedAt: mustParseTime(r.createdAt),
	}
	if r.defaultVal.Valid {
		v := r.defaultVal.String
		rule.DefaultJSON = &v
	}
	return rule, nil
}

// CreateRule 注册转换规则。规则必须绑定到目标契约。
func (s *Store) CreateRule(r model.TransformationRule) (model.TransformationRule, error) {
	if err := model.ValidateAction(r.Action); err != nil {
		return model.TransformationRule{}, err
	}
	if r.FromField == "" {
		return model.TransformationRule{}, fmt.Errorf("%w: from_field is required", model.ErrBadRequest)
	}
	var defVal any
	if r.DefaultJSON != nil {
		defVal = *r.DefaultJSON
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`
		INSERT INTO rules (contract_id, from_field, to_field, action, coerce_from, coerce_to,
			default_json, precedence, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		r.ContractID, r.FromField, r.ToField, r.Action, r.CoerceFrom, r.CoerceTo,
		defVal, r.Precedence, now)
	if err != nil {
		return model.TransformationRule{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetRule(id)
}

// GetRule 按 ID 读取规则。
func (s *Store) GetRule(id int64) (model.TransformationRule, error) {
	row := s.db.QueryRow(`
		SELECT id, contract_id, from_field, to_field, action, coerce_from, coerce_to,
			default_json, precedence, created_at FROM rules WHERE id = ?`, id)
	rule, err := scanRule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.TransformationRule{}, fmt.Errorf("%w: rule %d", model.ErrNotFound, id)
	}
	return rule, err
}

// ListRules 列出契约的全部转换规则（按 from_field, precedence）。
func (s *Store) ListRules(contractID int64) ([]model.TransformationRule, error) {
	rows, err := s.db.Query(`
		SELECT id, contract_id, from_field, to_field, action, coerce_from, coerce_to,
			default_json, precedence, created_at FROM rules WHERE contract_id = ?
		ORDER BY from_field, precedence`, contractID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.TransformationRule, 0)
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// DeleteRule 删除规则。
func (s *Store) DeleteRule(id int64) error {
	res, err := s.db.Exec(`DELETE FROM rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: rule %d", model.ErrNotFound, id)
	}
	return nil
}

// CountRules 统计契约的规则数量。
func (s *Store) CountRules(contractID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM rules WHERE contract_id = ?`, contractID).Scan(&n)
	return n, err
}
