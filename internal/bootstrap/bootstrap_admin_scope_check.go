package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	platformadmindomain "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	platformadmininfra "github.com/guliangiam/guliangiam/internal/infra/postgres/platform_admins"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 自检失败原因（用于区分“重播种子能修”与“重播种子修不了”）。
// bootstrapScopeExpiryWarningWindow is how early the startup check warns that
// the bootstrap all-tenant scope is about to expire. The scope expires after at
// most one year; renewing it is an operator action (see
// scripts/dev/renew-platform-admin-scope-bootstrap.sql).
const bootstrapScopeExpiryWarningWindow = 30 * 24 * time.Hour

const (
	bootstrapScopeReasonNoRow       = "no bootstrap row"
	bootstrapScopeReasonExpired     = "bootstrap scope expired"
	bootstrapScopeReasonFlagMissing = "bootstrap flag missing"
	bootstrapScopeReasonInactive    = "bootstrap admin inactive"
	bootstrapScopeReasonNoValid     = "no valid all-tenant scope"
)

// bootstrapAdminState 是 built-in 组织 admin 对应的平台管理员最小状态。
type bootstrapAdminState struct {
	ID       string
	IsActive bool
}

// bootstrapAdminQuery 解析 built-in 组织的 admin 状态。
// 未找到时返回 found=false（未播种的库是正常状态，不是启动失败）。
type bootstrapAdminQuery func(ctx context.Context) (admin bootstrapAdminState, found bool, err error)

// bootstrapScopeLister 列出某个 platform admin 的 scope。
// onlyActive=true 时必须走仓储的 scopeValidity（唯一权威谓词），
// onlyActive=false 仅用于诊断失败原因，不参与放行/拒绝判定。
type bootstrapScopeLister func(ctx context.Context, adminID string, onlyActive bool) ([]*platformadmindomain.PlatformAdminScope, error)

// queryBuiltinPlatformAdmin 从 platform_admins / users / organizations 读取
// built-in 组织 admin 的平台管理员 id 与启用状态。
//
// platform_admins 表受 RLS 保护（is_super_admin()），因此读取与
// EnsureBootstrapAdminCredential 使用同一条路径：事务内 set_config 局部
// super_admin，事务结束后随连接释放，不污染连接池会话。
func queryBuiltinPlatformAdmin(pool *pgxpool.Pool) bootstrapAdminQuery {
	const sql = `
		SELECT pa.id, pa.is_active
		FROM guliangiam.platform_admins pa
		JOIN guliangiam.users u ON u.id = pa.user_id
		JOIN guliangiam.organizations o ON o.id = u.organization_id
		WHERE u.username = 'admin'
		  AND o.slug = 'built-in'
		  AND u.deleted_at IS NULL
		  AND o.deleted_at IS NULL`

	return func(ctx context.Context) (bootstrapAdminState, bool, error) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return bootstrapAdminState{}, false, fmt.Errorf("begin bootstrap admin scope lookup: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		if _, err := tx.Exec(ctx, "SELECT set_config('app.current_role', 'super_admin', true)"); err != nil {
			return bootstrapAdminState{}, false, fmt.Errorf("set bootstrap admin scope lookup role: %w", err)
		}

		var admin bootstrapAdminState
		err = tx.QueryRow(ctx, sql).Scan(&admin.ID, &admin.IsActive)
		if errors.Is(err, pgx.ErrNoRows) {
			return bootstrapAdminState{}, false, nil
		}
		if err != nil {
			return bootstrapAdminState{}, false, fmt.Errorf("read built-in platform admin: %w", err)
		}
		return admin, true, nil
	}
}

// listBootstrapAdminScopes 复用 PlatformAdminScopeRepository 的列表查询。
// onlyActive=true 即 scopeValidity（expires_at > now() AND (is_bootstrap OR approved_by 非空)）。
func listBootstrapAdminScopes(pool *pgxpool.Pool) bootstrapScopeLister {
	repository := platformadmininfra.NewScopeRepository(pool)
	return func(ctx context.Context, adminID string, onlyActive bool) ([]*platformadmindomain.PlatformAdminScope, error) {
		return repository.ListByPlatformAdmin(ctx, adminID, platformadmindomain.ScopeListOptions{OnlyActive: onlyActive})
	}
}

// classifyBootstrapAllScope 只在“没有任何有效 all-scope”时用于生成原因。
// 是否有效由仓储 OnlyActive 查询（scopeValidity）判定；本函数不复制该谓词，
// 只检查各行的字段状态并映射到可执行动作。
func classifyBootstrapAllScope(scopes []*platformadmindomain.PlatformAdminScope, now time.Time) (reason string, expiredAt time.Time) {
	hasAllRow := false
	expired := false
	flagMissing := false
	for _, scope := range scopes {
		if scope == nil || scope.ScopeType != platformadmindomain.ScopeTypeAll {
			continue
		}
		hasAllRow = true
		if !scope.ExpiresAt.After(now) {
			if !expired || scope.ExpiresAt.After(expiredAt) {
				expiredAt = scope.ExpiresAt
			}
			expired = true
		}
		if !scope.IsBootstrap && scope.ApprovedBy == nil {
			flagMissing = true
		}
	}

	switch {
	case !hasAllRow:
		return bootstrapScopeReasonNoRow, time.Time{}
	case expired:
		return bootstrapScopeReasonExpired, expiredAt
	case flagMissing:
		return bootstrapScopeReasonFlagMissing, time.Time{}
	default:
		return bootstrapScopeReasonNoValid, time.Time{}
	}
}

// bootstrapScopeRemediation 把失败原因翻译成明确的可执行动作。
func bootstrapScopeRemediation(reason string, expiredAt time.Time) string {
	switch reason {
	case bootstrapScopeReasonNoRow:
		return "built-in admin exists but has no all-tenant scope row; run -seed on a fresh database, " +
			"or restore the row with scripts/dev/backfill-platform-admin-scope-bootstrap.sql, then restart"
	case bootstrapScopeReasonExpired:
		return fmt.Sprintf("the bootstrap all-tenant scope expired at %s; re-running -seed does NOT extend expires_at — "+
			"apply a backfill that sets a new expires_at, or decide whether the scope should have no expiry, then restart",
			expiredAt.UTC().Format(time.RFC3339))
	case bootstrapScopeReasonFlagMissing:
		return "the all-tenant scope has neither is_bootstrap nor approved_by; " +
			"apply scripts/dev/backfill-platform-admin-scope-bootstrap.sql, then restart"
	case bootstrapScopeReasonInactive:
		return "the built-in bootstrap administrator is inactive (is_active=false); enable the administrator, then restart"
	default:
		return "re-run seeding with -seed, or apply scripts/dev/backfill-platform-admin-scope-bootstrap.sql, then restart"
	}
}

// verifyBootstrapAdminAllScope 是启动期自检：seed 创建的 built-in 管理员必须
// 是启用状态且至少有一条有效 all-scope，否则平台侧跨租户可见性会静默消失
// （P1 症状）。
//
// 有效性判定完全来自 listScopes(..., onlyActive=true)（仓储 scopeValidity）；
// onlyActive=false 的全量列表只用于区分失败原因，不参与放行/拒绝。
//
// 生产环境 fail-fast：静默丢失平台全租户可见性正是 P1 的复现条件，不能在
// production 里带病起服。非生产环境只 WARN 并继续：本地/测试库可能故意处于
// 部分播种状态，不应把数据修复成本转嫁给每一次本地启动。
func verifyBootstrapAdminAllScope(
	ctx context.Context,
	resolveAdmin bootstrapAdminQuery,
	listScopes bootstrapScopeLister,
	production bool,
) error {
	admin, found, err := resolveAdmin(ctx)
	if err != nil {
		return apperrors.NewUnavailable("cannot verify bootstrap admin all-tenant scope: " + err.Error())
	}
	if !found {
		slog.Info("bootstrap admin scope check skipped",
			"reason", "built-in organization admin is not present (database not seeded)",
		)
		return nil
	}

	if !admin.IsActive {
		return reportBootstrapScopeFailure(ctx, production, admin.ID,
			bootstrapScopeReasonInactive, time.Time{})
	}

	activeScopes, err := listScopes(ctx, admin.ID, true)
	if err != nil {
		return apperrors.NewUnavailable("cannot read bootstrap admin scopes: " + err.Error())
	}
	for _, scope := range activeScopes {
		if scope.ScopeType != platformadmindomain.ScopeTypeAll {
			continue
		}
		slog.Info("bootstrap admin all-tenant scope verified",
			"platform_admin_id", admin.ID,
			"scope_id", scope.ID,
			"is_bootstrap", scope.IsBootstrap,
			"approved", scope.ApprovedBy != nil,
			"expires_at", scope.ExpiresAt,
		)
		dataScope, expiresAt, remaining := expiringBootstrapScope(activeScopes, time.Now())
		if dataScope != "" {
			slog.Warn("bootstrap admin all-tenant scope expires soon; renew it before it lapses",
				"platform_admin_id", admin.ID,
				"scope_id", dataScope,
				"expires_at", expiresAt,
				"remaining_hours", int64(remaining.Hours()),
				"renewal_script", "scripts/dev/renew-platform-admin-scope-bootstrap.sql",
			)
		}
		return nil
	}

	allScopes, err := listScopes(ctx, admin.ID, false)
	if err != nil {
		return apperrors.NewUnavailable("cannot read bootstrap admin scopes: " + err.Error())
	}
	reason, expiredAt := classifyBootstrapAllScope(allScopes, time.Now())
	return reportBootstrapScopeFailure(ctx, production, admin.ID, reason, expiredAt)
}

// reportBootstrapScopeFailure 统一输出原因与可执行动作：
// production 返回 Unavailable（启动 fail-fast），非生产 WARN 后继续。
func reportBootstrapScopeFailure(
	ctx context.Context,
	production bool,
	adminID string,
	reason string,
	expiredAt time.Time,
) error {
	remediation := bootstrapScopeRemediation(reason, expiredAt)

	if production {
		resourceType := "built-in platform admin all-tenant scope"
		message := resourceType + " check failed: reason=" + reason + "; " + remediation
		return apperrors.NewUnavailable(message)
	}

	slog.WarnContext(ctx, "bootstrap admin scope check failed; continuing because APP_ENV is not production",
		"platform_admin_id", adminID,
		"reason", reason,
		"remediation", remediation,
		"expires_at", expiredAt,
	)
	return nil
}

// expiringBootstrapScope returns the earliest active all-tenant scope that
// expires inside bootstrapScopeExpiryWarningWindow. Empty scopeID means no
// action is needed.
func expiringBootstrapScope(scopes []*platformadmindomain.PlatformAdminScope, now time.Time) (scopeID string, expiresAt time.Time, remaining time.Duration) {
	for _, scope := range scopes {
		if scope == nil || scope.ScopeType != platformadmindomain.ScopeTypeAll || scope.ExpiresAt.IsZero() {
			continue
		}
		left := scope.ExpiresAt.Sub(now)
		if left > bootstrapScopeExpiryWarningWindow {
			continue
		}
		if scopeID == "" || scope.ExpiresAt.Before(expiresAt) {
			scopeID = scope.ID
			expiresAt = scope.ExpiresAt
			remaining = left
		}
	}
	return scopeID, expiresAt, remaining
}
