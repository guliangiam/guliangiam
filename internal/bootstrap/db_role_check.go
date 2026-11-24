package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// postgresRoleAttributes 是启动自检读取的当前连接角色属性。
type postgresRoleAttributes struct {
	Name      string
	Super     bool
	BypassRLS bool
}

// postgresRoleQuery 是可注入的角色属性查询，便于单元测试。
type postgresRoleQuery func(ctx context.Context) (postgresRoleAttributes, error)

// queryPostgresRoleAttributes 从 pg_roles 读取 current_user 的 rolsuper/rolbypassrls。
func queryPostgresRoleAttributes(pool *pgxpool.Pool) postgresRoleQuery {
	return func(ctx context.Context) (postgresRoleAttributes, error) {
		var attributes postgresRoleAttributes
		err := pool.QueryRow(ctx,
			`SELECT rolname, rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user`,
		).Scan(&attributes.Name, &attributes.Super, &attributes.BypassRLS)
		if err != nil {
			return postgresRoleAttributes{}, fmt.Errorf("query current role attributes: %w", err)
		}
		return attributes, nil
	}
}

// verifyPostgresRoleIsolation 在 PostgreSQL 启动期校验连接角色必须
// rolsuper=false 且 rolbypassrls=false；否则 RLS 租户隔离形同虚设，直接 fail-fast。
// 错误信息只输出角色名与属性，不输出 DSN。
func verifyPostgresRoleIsolation(ctx context.Context, query postgresRoleQuery) error {
	attributes, err := query(ctx)
	if err != nil {
		return apperrors.NewUnavailable("cannot verify PostgreSQL role isolation attributes: " + err.Error())
	}
	if attributes.Super || attributes.BypassRLS {
		return apperrors.NewUnavailable(fmt.Sprintf(
			"database role %q is not RLS-safe (rolsuper=%t, rolbypassrls=%t); "+
				"tenant isolation requires a role with rolsuper=false and rolbypassrls=false",
			attributes.Name, attributes.Super, attributes.BypassRLS,
		))
	}

	slog.Info("postgres role isolation verified",
		"role", attributes.Name,
		"rolsuper", attributes.Super,
		"rolbypassrls", attributes.BypassRLS,
	)
	return nil
}
