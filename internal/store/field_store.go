package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"task261-apimigproof/internal/model"
)

// fieldRow 是 fields 表的一行。
type fieldRow struct {
	id          int64
	contractID  int64
	fieldID     string
	status      string
	valueType   string
	hasDefault  int
	defaultVal  sql.NullString
	deprecatedIn sql.NullInt64
	removedIn   sql.NullInt64
	description string
	updatedAt   string
}

func scanField(row interface{ Scan(...interface{}) error }) (model.FieldSemantics, error) {
	var r fieldRow
	if err := row.Scan(&r.id, &r.contractID, &r.fieldID, &r.status, &r.valueType,
		&r.hasDefault, &r.defaultVal, &r.deprecatedIn, &r.removedIn, &r.description, &r.updatedAt); err != nil {
		return model.FieldSemantics{}, err
	}
	f := model.FieldSemantics{
		ID: r.id, ContractID: r.contractID, FieldID: r.fieldID,
		Status: r.status, ValueType: r.valueType,
		HasDefault: r.hasDefault == 1, Description: r.description,
		UpdatedAt: mustParseTime(r.updatedAt),
	}
	if r.defaultVal.Valid {
		v := r.defaultVal.String
		f.DefaultValue = &v
	}
	if r.deprecatedIn.Valid {
		v := int(r.deprecatedIn.Int64)
		f.DeprecatedIn = &v
	}
	if r.removedIn.Valid {
		v := int(r.removedIn.Int64)
		f.RemovedIn = &v
	}
	return f, nil
}

// UpsertField 新增或更新字段语义。字段 ID 在同一契约内唯一（更新走同一条）。
func (s *Store) UpsertField(f model.FieldSemantics) (model.FieldSemantics, error) {
	if f.FieldID == "" || strings.ContainsAny(f.FieldID, " \t\n") {
		return model.FieldSemantics{}, fmt.Errorf("%w: invalid field id %q", model.ErrBadRequest, f.FieldID)
	}
	if err := model.ValidateFieldStatus(f.Status); err != nil {
		return model.FieldSemantics{}, err
	}
	if err := model.ValidateValueType(f.ValueType); err != nil {
		return model.FieldSemantics{}, err
	}
	var defVal any
	if f.HasDefault && f.DefaultValue != nil {
		defVal = *f.DefaultValue
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO fields (contract_id, field_id, status, value_type, has_default, default_value,
			deprecated_in, removed_in, description, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT (contract_id, field_id) DO UPDATE SET
			status = excluded.status,
			value_type = excluded.value_type,
			has_default = excluded.has_default,
			default_value = excluded.default_value,
			deprecated_in = excluded.deprecated_in,
			removed_in = excluded.removed_in,
			description = excluded.description,
			updated_at = excluded.updated_at`,
		f.ContractID, f.FieldID, f.Status, f.ValueType, boolInt(f.HasDefault), defVal,
		f.DeprecatedIn, f.RemovedIn, f.Description, now,
	)
	if err != nil {
		return model.FieldSemantics{}, err
	}
	return s.GetField(f.ContractID, f.FieldID)
}

// GetField 按契约与字段 ID 读取字段语义。
func (s *Store) GetField(contractID int64, fieldID string) (model.FieldSemantics, error) {
	row := s.db.QueryRow(`
		SELECT id, contract_id, field_id, status, value_type, has_default, default_value,
			deprecated_in, removed_in, description, updated_at
		FROM fields WHERE contract_id = ? AND field_id = ?`, contractID, fieldID)
	f, err := scanField(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.FieldSemantics{}, fmt.Errorf("%w: field %q in contract %d", model.ErrUnknownField, fieldID, contractID)
	}
	return f, err
}

// ListFields 列出契约的全部字段语义。
func (s *Store) ListFields(contractID int64) ([]model.FieldSemantics, error) {
	rows, err := s.db.Query(`
		SELECT id, contract_id, field_id, status, value_type, has_default, default_value,
			deprecated_in, removed_in, description, updated_at
		FROM fields WHERE contract_id = ? ORDER BY field_id`, contractID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.FieldSemantics, 0)
	for rows.Next() {
		f, err := scanField(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// DeleteField 从契约中删除字段语义（仅 draft 契约允许）。
func (s *Store) DeleteField(contractID int64, fieldID string) error {
	res, err := s.db.Exec(`DELETE FROM fields WHERE contract_id = ? AND field_id = ?`, contractID, fieldID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: field %q in contract %d", model.ErrUnknownField, fieldID, contractID)
	}
	return nil
}

// CountFieldsByStatus 统计契约内指定状态的字段数量。
func (s *Store) CountFieldsByStatus(contractID int64, status string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM fields WHERE contract_id = ? AND status = ?`,
		contractID, status).Scan(&n)
	return n, err
}
