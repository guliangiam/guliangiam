// Package shared 提供 MySQL 存储层各 repo 共享的基础设施：
//   - DBTX 抽象 *sql.DB 和 *sql.Tx 的公共方法
//   - 分页辅助、JSON 编解码等通用工具
package shared

import (
	"context"
	"database/sql"
	"encoding/json"

	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// DBTX 是 *sql.DB 和 *sql.Tx 的最小公共接口。
// 所有 MySQL repo 的 NewRepository 都接受这个接口，让 repo 能在事务内复用同一个实现。
type DBTX interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Beginner 是能开启事务的东西（*sql.DB）。
// *sql.Tx 不实现 Beginner（嵌套事务用 savepoint），所以单独声明。
type Beginner interface {
	DBTX
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// MarshalJSON 与 PG 版 shared.MarshalJSON 同构，避免引入 PG 依赖。
func MarshalJSON(v any) ([]byte, error) { return json.Marshal(v) }

// UnmarshalJSON 与 PG 版 shared.UnmarshalJSON 同构。
func UnmarshalJSON(data []byte, v any) error { return json.Unmarshal(data, v) }

// ClampPageSize 限制单页上限。
func ClampPageSize(size int) int {
	const maxSize = 500
	if size <= 0 {
		return 20
	}
	if size > maxSize {
		return maxSize
	}
	return size
}

// PageOffset 把 (page, pageSize) 转成 SQL OFFSET，page 从 1 起。
//
// page 先用 platform/pagination 的上界收紧，再与本地 500 上限的 pageSize 相乘；
// 最坏乘积是 (MaxPage-1)*500，仍在 32 位 int 范围内，因此不会回绕成负 offset。
// pageSize 上限保持 MySQL 镜像的 500（不套用 HTTP 的 100），以免改变镜像的
// LIMIT 语义。
func PageOffset(page, pageSize int) int {
	return (platformpagination.ClampPage(page) - 1) * ClampPageSize(pageSize)
}
