package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"task261-apimigproof/internal/model"
)

// contractRow 是 contracts 表的一行。
type contractRow struct {
	id        int64
	name      string
	version   int
	status    string
	createdAt string
	sealedAt  sql.NullString
}

func scanContract(row interface{ Scan(...interface{}) error }) (model.ContractVersion, error) {
	var r contractRow
	if err := row.Scan(&r.id, &r.name, &r.version, &r.status, &r.createdAt, &r.sealedAt); err != nil {
		return model.ContractVersion{}, err
	}
	c := model.ContractVersion{
		ID: r.id, Name: r.name, Version: r.version, Status: r.status,
		CreatedAt: mustParseTime(r.createdAt),
	}
	if r.sealedAt.Valid {
		t := mustParseTime(r.sealedAt.String)
		c.SealedAt = &t
	}
	return c, nil
}

// CreateContract 创建契约版本。同名契约的版本号必须严格递增。
func (s *Store) CreateContract(name string, version int) (model.ContractVersion, error) {
	var exists int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM contracts WHERE name = ? AND version = ?`, name, version,
	).Scan(&exists); err != nil {
		return model.ContractVersion{}, err
	}
	if exists > 0 {
		return model.ContractVersion{}, fmt.Errorf("%w: contract %s v%d already exists", model.ErrConflict, name, version)
	}
	// 版本倒退检查：若该契约已存在更高版本则拒绝。
	var maxVer int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM contracts WHERE name = ?`, name).Scan(&maxVer); err != nil {
		return model.ContractVersion{}, err
	}
	if version <= maxVer {
		return model.ContractVersion{}, fmt.Errorf("%w: %s already at v%d, cannot create v%d",
			model.ErrVersionRegression, name, maxVer, version)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO contracts (name, version, status, created_at, sealed_at) VALUES (?,?,?,?,NULL)`,
		name, version, model.ContractDraft, now,
	)
	if err != nil {
		return model.ContractVersion{}, err
	}
	id, _ := res.LastInsertId()
	return model.ContractVersion{
		ID: id, Name: name, Version: version, Status: model.ContractDraft, CreatedAt: mustParseTime(now),
	}, nil
}

// GetContract 按 ID 读取契约版本。
func (s *Store) GetContract(id int64) (model.ContractVersion, error) {
	row := s.db.QueryRow(
		`SELECT id, name, version, status, created_at, sealed_at FROM contracts WHERE id = ?`, id,
	)
	c, err := scanContract(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ContractVersion{}, fmt.Errorf("%w: contract %d", model.ErrNotFound, id)
	}
	if err != nil {
		return model.ContractVersion{}, err
	}
	c.FieldCount, _ = s.CountFields(id)
	return c, nil
}

// ListContracts 列出全部契约版本。
func (s *Store) ListContracts() ([]model.ContractVersion, error) {
	rows, err := s.db.Query(`SELECT id, name, version, status, created_at, sealed_at FROM contracts ORDER BY name, version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.ContractVersion, 0)
	for rows.Next() {
		c, err := scanContract(rows)
		if err != nil {
			return nil, err
		}
		c.FieldCount, _ = s.CountFields(c.ID)
		out = append(out, c)
	}
	return out, rows.Err()
}

// CountFields 统计契约的字段数量。
func (s *Store) CountFields(contractID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM fields WHERE contract_id = ?`, contractID).Scan(&n)
	return n, err
}

// SetContractStatus 更新契约状态。
func (s *Store) SetContractStatus(id int64, status string) error {
	_, err := s.db.Exec(`UPDATE contracts SET status = ? WHERE id = ?`, status, id)
	return err
}

// SealContract 封存契约：置 sealed 并记录封存时间。
func (s *Store) SealContract(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`UPDATE contracts SET status = ?, sealed_at = ? WHERE id = ?`, model.ContractSealed, now, id)
	return err
}
