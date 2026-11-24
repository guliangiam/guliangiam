// Package casbin 提供 Casbin 策略存储的 MySQL 方言实现（ADR-023 第二基线）。
//
// 与 PostgreSQL 版（internal/infra/casbin.Adapter）保持同一 PolicyStore 端口：
// 读写 casbin_rules、按 enforcer_id 作用域、JIT 时间窗在 Go 内求值（MySQL 的
// v4/v5 是 VARCHAR，避免依赖 JSON/TIMESTAMP 的方言差异）。
package casbin

import (
	casbinport "github.com/guliangiam/guliangiam/internal/infra/casbin"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

// Store 实现 casbinport.PolicyStore，基于 database/sql（*sql.DB 或 *sql.Tx）。
type Store struct {
	db         mysqlshared.DBTX
	enforcerID string
}

// NewStore 创建 MySQL 策略存储。
func NewStore(db mysqlshared.DBTX) Store {
	return Store{db: db}
}

// Scoped 返回绑定到指定 enforcer 记录的副本。
func (s Store) Scoped(enforcerID string) casbinport.PolicyStore {
	return Store{db: s.db, enforcerID: enforcerID}
}

var _ casbinport.PolicyStore = Store{}
