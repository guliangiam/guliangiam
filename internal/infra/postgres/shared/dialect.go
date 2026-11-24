// Package shared 提供 PostgreSQL 存储层各 repo 共享的基础设施。
//
// Dialect: 本文件暴露 postgres 的 Dialect 单例，所有仓库通过 shared.Dialect
// 生成方言 SQL 片段（ON CONFLICT, LIMIT, FOR UPDATE SKIP LOCKED, ...）。
// 进程内只允许一个 Dialect，由 import 路径在编译期锁定（infra/postgres →
// PostgresDialect；infra/mysql → MySQLDialect）。
package shared

import (
	"github.com/guliangiam/guliangiam/internal/infra/dialect"
)

// Dialect 是 PostgreSQL 路径的方言单例。
// 所有 infra/postgres 下的 sql.go / repository.go 都通过本变量生成
// 方言专有 SQL 片段，禁止直接硬写 ON CONFLICT, $1::uuid, SKIP LOCKED 等。
//
// 单测可临时替换为 mock（SetDialectForTest），但生产代码绝不应修改。
var Dialect dialect.Dialect = dialect.PostgresDialect{}

// SetDialectForTest 仅用于单元测试替换 Dialect。
// 返回恢复函数，用于 t.Cleanup 里恢复原 Dialect。
//
//	restore := shared.SetDialectForTest(mockDialect{})
//	t.Cleanup(restore)
func SetDialectForTest(d dialect.Dialect) func() {
	orig := Dialect
	Dialect = d
	return func() { Dialect = orig }
}

// GetDialectForTest 返回当前 Dialect（断言 helper）。
func GetDialectForTest() dialect.Dialect { return Dialect }
