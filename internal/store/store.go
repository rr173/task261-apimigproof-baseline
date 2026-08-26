// Package store 提供基于 SQLite 的持久化层。
//
// 使用纯 Go 驱动 modernc.org/sqlite（CGO_ENABLED=0 可离线构建），
// 全部业务数据（契约、字段语义、转换规则、样本、兼容窗口、比较、证明）
// 落盘保存，进程重启后完整恢复。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store 封装 SQLite 连接与全部数据访问子仓库。
type Store struct {
	db *sql.DB
}

// OpenStore 打开（或创建）SQLite 数据库并执行建表迁移。
func OpenStore(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close 关闭底层连接。
func (s *Store) Close() error { return s.db.Close() }

// DB 暴露底层连接，供服务层做事务编排。
func (s *Store) DB() *sql.DB { return s.db }

// migrate 执行建表迁移（幂等：CREATE TABLE IF NOT EXISTS）。
func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS contracts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			version INTEGER NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			sealed_at TEXT,
			UNIQUE (name, version)
		)`,
		`CREATE TABLE IF NOT EXISTS fields (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			contract_id INTEGER NOT NULL REFERENCES contracts(id),
			field_id TEXT NOT NULL,
			status TEXT NOT NULL,
			value_type TEXT NOT NULL,
			has_default INTEGER NOT NULL DEFAULT 0,
			default_value TEXT,
			deprecated_in INTEGER,
			removed_in INTEGER,
			description TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL,
			UNIQUE (contract_id, field_id)
		)`,
		`CREATE TABLE IF NOT EXISTS rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			contract_id INTEGER NOT NULL REFERENCES contracts(id),
			from_field TEXT NOT NULL,
			to_field TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			coerce_from TEXT NOT NULL DEFAULT '',
			coerce_to TEXT NOT NULL DEFAULT '',
			default_json TEXT,
			precedence INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			fingerprint TEXT NOT NULL UNIQUE,
			payload_json TEXT NOT NULL,
			status TEXT NOT NULL,
			imported_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS windows (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_contract_id INTEGER NOT NULL REFERENCES contracts(id),
			to_contract_id INTEGER NOT NULL REFERENCES contracts(id),
			policy TEXT NOT NULL,
			valid_until TEXT,
			note TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS comparisons (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_contract_id INTEGER NOT NULL REFERENCES contracts(id),
			to_contract_id INTEGER NOT NULL REFERENCES contracts(id),
			status TEXT NOT NULL,
			total_samples INTEGER NOT NULL DEFAULT 0,
			migratable INTEGER NOT NULL DEFAULT 0,
			rejected INTEGER NOT NULL DEFAULT 0,
			semantics_changed INTEGER NOT NULL DEFAULT 0,
			window_id INTEGER,
			created_at TEXT NOT NULL,
			finished_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS comparison_results (
			comparison_id INTEGER NOT NULL REFERENCES comparisons(id),
			sample_id INTEGER NOT NULL REFERENCES samples(id),
			verdict TEXT NOT NULL,
			issue TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (comparison_id, sample_id)
		)`,
		`CREATE TABLE IF NOT EXISTS comparison_issues (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			comparison_id INTEGER NOT NULL REFERENCES comparisons(id),
			sample_id INTEGER NOT NULL,
			kind TEXT NOT NULL,
			field TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS proofs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_contract_id INTEGER NOT NULL REFERENCES contracts(id),
			to_contract_id INTEGER NOT NULL REFERENCES contracts(id),
			comparison_id INTEGER NOT NULL REFERENCES comparisons(id),
			status TEXT NOT NULL,
			summary_json TEXT NOT NULL,
			evidence_fingerprint TEXT NOT NULL,
			created_at TEXT NOT NULL,
			published_at TEXT,
			superseded_by INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_fields_contract ON fields(contract_id)`,
		`CREATE INDEX IF NOT EXISTS idx_rules_contract ON rules(contract_id)`,
		`CREATE INDEX IF NOT EXISTS idx_comparisons_pair ON comparisons(from_contract_id, to_contract_id)`,
		`CREATE INDEX IF NOT EXISTS idx_proofs_pair ON proofs(from_contract_id, to_contract_id)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec migrate: %w", err)
		}
	}
	return nil
}
