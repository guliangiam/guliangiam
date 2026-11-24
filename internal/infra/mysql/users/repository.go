package users

import (
	"context"
	"database/sql"
	"strings"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

// Repository 是 MySQL 存储层的 users 仓库。
// 与 internal/infra/postgres/users.Repository 实现同域接口，
// 由 mysqluow 在 runtime.go 中按 driver 选中。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

var _ domain.Repository = Repository{}

// rowsAffected 从 sql.Result 中安全提取影响行数。
func rowsAffected(r sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// withinTransaction 在 r.db 上开一个真实 MySQL 子事务（或 savepoint）执行 fn。
// 如果 r.db 不是 Beginner（例如已经是 *sql.Tx），直接调用 fn。
func (r Repository) withinTransaction(ctx context.Context, fn func(ctx context.Context, tx mysqlshared.DBTX) error) error {
	b, ok := r.db.(mysqlshared.Beginner)
	if !ok {
		return fn(ctx, r.db)
	}
	tx, err := b.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// nullableString 把空串变成 nil，让 MySQL 写 NULL。
func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// nullTimeArg 把 *time.Time 转成可绑定值（nil 或 time.Time）。
func nullTimeArg(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

// inPlaceholders 生成 `?, ?, ?, ...`（n 个）。
func inPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?, ", n-1) + "?"
}

// replaceInPlaceholders 把 SQL 模板里的 PLACEHOLDER_IDS 替换成 ? 列表。
func replaceInPlaceholders(sqlTpl string, n int) string {
	return strings.Replace(sqlTpl, "PLACEHOLDER_IDS", inPlaceholders(n), 1)
}

// ── Read ────────────────────────────────────────────────────────────────
