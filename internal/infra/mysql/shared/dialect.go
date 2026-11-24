// Package shared 提供 MySQL 存储层各 repo 共享的基础设施。
//
// Dialect: 本文件暴露 mysql 的 Dialect 单例，所有仓库通过 shared.Dialect
// 生成方言 SQL 片段（ON DUPLICATE KEY UPDATE, LIMIT offset,count, ...）。
// 进程内只允许一个 Dialect，由 import 路径在编译期锁定。
//
// 参见 postgres/shared/dialect.go 对生产使用约束的完整注释。
package shared

import (
	"github.com/guliangiam/guliangiam/internal/infra/dialect"
)

// Dialect 是 MySQL 路径的方言单例。
// 所有 infra/mysql 下的 sql.go / repository.go 都通过本变量生成
// 方言专有 SQL 片段，禁止直接硬写 ON DUPLICATE KEY UPDATE, FOR UPDATE SKIP LOCKED 等。
var Dialect dialect.Dialect = dialect.MySQLDialect{}

// SetDialectForTest 仅用于单元测试替换 Dialect。返回恢复函数给 t.Cleanup。
func SetDialectForTest(d dialect.Dialect) func() {
	orig := Dialect
	Dialect = d
	return func() { Dialect = orig }
}

// GetDialectForTest 返回当前 Dialect（断言 helper）。
func GetDialectForTest() dialect.Dialect { return Dialect }
