package db

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureDatabaseExists 在真正的 pgxpool 创建之前保证 DSN 指向的目标库存在。
//
// 机制：
//  1. 从目标 DSN 解析出 host/port/user/password/sslmode
//  2. 把 database 字段替换为 "postgres"（PG 管理库，总是存在）
//  3. 用这条 DSN 建立一次临时 pgx.Connect 到管理库
//  4. 查 pg_database 系统表判断目标库是否存在
//  5. 不存在 → 执行 CREATE DATABASE "<目标库>" OWNER "<当前连接用户>"
//  6. 关闭临时连接，把 DSN 原样交给上层创建 pgxpool
//
// 为什么用单连接而非 pool：
//   - CREATE DATABASE 必须 autocommit，pgx.Connect 默认就是 autocommit
//   - pgxpool 的 AfterConnect 钩子不应该在管理库建库场景跑（会 RLS fail-closed）
//
// 为什么 OWNER 取当前连接用户：
//   - pgx.ConnConfig.User 就是 DSN 里的 user（postgres://user:pass@host/db 里的 user）
//   - 本地开发 DSN 是 postgres://guliangiam:guliangiam@localhost/guliangiam → owner 自动是 guliangiam
//   - 云托管 DSN 可能是 postgres://admin:xxx@host/db → 但云托管不允许普通用户 CREATE DATABASE，
//     这种场景会在 CREATE DATABASE 时得到 permission denied，函数会 log 并返回原始错误
func EnsureDatabaseExists(ctx context.Context, targetDSN string) error {
	if targetDSN == "" {
		return apperrors.NewNotFound("postgres DSN is empty")
	}

	targetCfg, err := pgxpool.ParseConfig(targetDSN)
	if err != nil {
		return apperrors.WrapInternalf(err, "parse target DSN")
	}

	targetDB := targetCfg.ConnConfig.Database
	if targetDB == "" {
		return apperrors.NewNotFound("target database name is empty in DSN")
	}

	// 构造管理库连接：除 database 字段替换成 "postgres"，其余全部保留
	adminCfg := targetCfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"

	adminConn, err := pgx.ConnectConfig(ctx, adminCfg.ConnConfig)
	if err != nil {
		return apperrors.WrapInternalf(err, "connect to postgres admin database")
	}
	defer adminConn.Close(ctx)

	// 检查目标库是否已存在
	var exists bool
	err = adminConn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)",
		targetDB,
	).Scan(&exists)
	if err != nil {
		return apperrors.WrapInternalf(err, "check target database existence")
	}
	if exists {
		slog.Info("target postgres database already exists", "database", targetDB)
		return nil
	}

	// 执行 CREATE DATABASE — 标识符必须用双引号防止关键字冲突，用 QuoteIdentifier 安全拼接
	quotedDB := quotePGIdentifier(targetDB)
	owner := adminCfg.ConnConfig.User
	createSQL := fmt.Sprintf(`CREATE DATABASE %s OWNER %s`, quotedDB, quotePGIdentifier(owner))

	slog.Info("target postgres database missing, creating it now",
		"database", targetDB, "owner", owner)

	tag, err := adminConn.Exec(ctx, createSQL)
	if err != nil {
		// 并发 race：两个进程同时启动都发现库不存在，第二个会得到 duplicate key
		var pgErr *pgconn.PgError
		if strings.Contains(err.Error(), "already exists") ||
			(strings.Contains(err.Error(), "duplicate key") && strings.Contains(err.Error(), "pg_database")) {
			slog.Info("database already created by another process", "database", targetDB)
			return nil
		}
		if pgErr != nil {
			slog.Warn("CREATE DATABASE failed — may need manual creation or privileged user",
				"database", targetDB, "pg_error", pgErr.Message, "hint", "ensure $PGUSER has CREATEDB privilege or run as superuser once")
		}
		return apperrors.WrapInternalf(err, "CREATE DATABASE %s", targetDB)
	}

	slog.Info("postgres database created successfully",
		"database", targetDB, "owner", owner, "command_tag", tag.String())
	return nil
}

// quotePGIdentifier 用双引号安全拼接 PostgreSQL 标识符，避免关键字冲突和大小写折叠。
// pgconn.Identifier 和 pgx/v5 的 Identifier 类型可直接用，但这里保持最小依赖。
func quotePGIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

type PostgresConfig struct {
	DataSourceName    string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// NewPostgresPool 创建 pgxpool + 三层 RLS 防护：
//
//  1. AfterConnect — fail-closed 默认（role=”, org=”）
//  2. PrepareConn — 每次 pool.Acquire(ctx) 前从 ctx 读 TenantScope，
//     设置正确的 role + org_id（SESSION=false 级）。覆盖所有路径：
//     pool.QueryRow / pool.Exec / pool.Acquire / UOW.WithinTransaction。
//     事务内的 WithinTransaction 会用 LOCAL=true 覆盖，提交后恢复 SESSION 值。
//  3. AfterRelease — 连接归还 pool 前 reset 回 fail-closed，
//     防止 SESSION GUC 在不同请求间泄漏。
//
// ⚠️ 为何用 PrepareConn 而非 BeforeQuery：
//   - BeforeQuery 每次 SQL 调用都触发（事务内可能 N 次），PrepareConn 只在 Acquire 时触发一次
//   - PrepareConn 的 ctx 就是 pool.Acquire(ctx) 传入的 ctx（含 TenantScope），
//     BeforeQuery 的 ctx 是单次 Query 的 ctx（同一个但粒度更细）
//   - PrepareConn 是 pgx v5.11 官方推荐钩子（BeforeAcquire 已 Deprecated）
func NewPostgresPool(ctx context.Context, cfg PostgresConfig) (*pgxpool.Pool, error) {
	if cfg.DataSourceName == "" {
		return nil, apperrors.NewNotFound("postgres data source name is empty")
	}

	// 前置：保证目标库已存在（PostgreSQL 不会自动 CREATE DATABASE —— pgxpool 连
	// 不存在的库会直接报 "database ... does not exist"）。用管理库（postgres）建
	// 临时单连接做一次检查，不存在就 CREATE DATABASE 后再进入正常 pool 流程。
	if err := EnsureDatabaseExists(ctx, cfg.DataSourceName); err != nil {
		return nil, err
	}

	parsed, err := pgxpool.ParseConfig(cfg.DataSourceName)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse postgres config")
	}

	if cfg.MaxConns > 0 {
		parsed.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		parsed.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		parsed.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		parsed.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.HealthCheckPeriod > 0 {
		parsed.HealthCheckPeriod = cfg.HealthCheckPeriod
	}

	parsed.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement
	parsed.ConnConfig.RuntimeParams["statement_timeout"] = "10000"

	// AfterConnect: 新物理连接建立 → fail-closed 默认
	parsed.AfterConnect = SetDefaultRLSForPoolAfterConnect

	// slug → UUID 解析缓存（PrepareConn 内部用）
	// 同一个 slug 在不同请求间不变 → 60s TTL 足够覆盖所有活跃请求
	slugCache := newSlugUUIDResolver(60 * time.Second)

	// PrepareConn: 每次 pool.Acquire 前 → 从 ctx 读 TenantScope → set_config SESSION
	parsed.PrepareConn = func(acquireCtx context.Context, conn *pgx.Conn) (bool, error) {
		if err := prepareRLSConn(acquireCtx, conn, slugCache); err != nil {
			return false, err
		}
		return true, nil
	}

	// AfterRelease: 连接归还 pool → reset 回 fail-closed（防御 GUC 泄漏）
	parsed.AfterRelease = func(conn *pgx.Conn) bool {
		// reset 失败只能返回 false 销毁连接 — 否则下一个请求拿到带 GUC 的连接会泄漏隔离
		_, err := conn.Exec(context.Background(),
			`SELECT set_config($1, '', false), set_config($2, '', false)`,
			RLS_GUCRole, RLSGUCOrganizationID,
		)
		return err == nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, parsed)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create postgres pool")
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, apperrors.WrapInternalf(err, "ping postgres pool")
	}

	return pool, nil
}

// rlsRoleForMode 把租户过滤模式映射为 RLS 业务 role。
// 只有显式 super_admin/system 才提升权限，其余（含 strict/optional）一律 member。
func rlsRoleForMode(mode tenantctx.TenantFilterMode) string {
	switch mode {
	case tenantctx.TenantFilterSuperAdmin, tenantctx.TenantFilterSystem:
		return RLSRoleSuperAdmin
	case tenantctx.TenantFilterStrict, tenantctx.TenantFilterOptional, tenantctx.TenantFilterCross:
		return RLSRoleMember
	}
	return RLSRoleMember
}

// prepareRLSConn 在 PrepareConn 钩子里设置 RLS GUC。
// ⚠️ 必须用 SESSION(false)，不能用 LOCAL(true)：
// PrepareConn 不在任何事务里，LOCAL 设置会立即丢失。
//
// 执行流程：
//  1. 先切 super_admin → 做 slug→UUID 解析（需要 RLS 放行 organizations）
//  2. 设回正确 role + org_id → 后续所有 SQL 用正确隔离
//
// 为什么 PrepareConn 而非 BeforeQuery：
//   - PrepareConn 每次 Acquire 只触发一次；BeforeQuery 每次 SQL 都触发（事务内 N 次）
//   - PrepareConn 的 ctx 就是 pool.Acquire(ctx) 传入的 request ctx
//     （TenantScope 在 TenantFilterMiddleware 里已注入）
func prepareRLSConn(ctx context.Context, conn *pgx.Conn, resolver *slugUUIDResolver) error {
	scope := tenantctx.TenantScopeFromContext(ctx)

	// 1. 映射业务 role
	role := rlsRoleForMode(scope.Mode)

	// 2. slug → UUID 解析（需要 super_admin 查 organizations FORCE RLS）
	var orgID string
	if role != RLSRoleSuperAdmin {
		raw := scope.OrganizationID
		if raw == "" {
			raw = scope.OrganizationName
		}
		if raw != "" {
			if isValidUUID(raw) {
				orgID = raw
			} else {
				// 切 super_admin 做查询（SESSION=false）
				if _, err := conn.Exec(ctx, `SELECT set_config('app.current_role', 'super_admin', false)`); err != nil {
					return err
				}
				if uuid, err := resolver.Resolve(ctx, conn, raw); err == nil {
					orgID = uuid
				}
				// Resolve 可能命中缓存（没查 DB）也可能查了 DB — 两种情况都 OK，
				// 因为 prepareRLSConn 阶段 3 会无条件设回正确 role
			}
		}
	}

	// 3. 最终设正确的 role + org_id（SESSION=false）
	// 空字符串 → current_organization_id() = NULLIF('', '')::uuid = NULL
	_, err := conn.Exec(ctx,
		`SELECT set_config($1, $2, false), set_config($3, $4, false)`,
		RLS_GUCRole, role,
		RLSGUCOrganizationID, orgID,
	)
	return err
}

// isValidUUID 不依赖外部库的 UUID 格式校验。
func isValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// slugUUIDResolver — slug→UUID 带 TTL 的内存缓存。
// 构造时传入 PrepareConn 闭包引用的 *pgx.Conn 接口，
// 内部用 super_admin role 查询 organizations 表。
type slugUUIDResolver struct {
	mu    sync.RWMutex
	cache map[string]cacheEntry
	ttl   time.Duration
}

type cacheEntry struct {
	uuid     string
	expires  time.Time
	negative bool // 找不到的 slug 也缓存（避免每次 Acquire 都查）
}

func newSlugUUIDResolver(ttl time.Duration) *slugUUIDResolver {
	return &slugUUIDResolver{
		cache: make(map[string]cacheEntry),
		ttl:   ttl,
	}
}

// Resolve 从 slug → UUID（带缓存）。找不到返回 ("", nil) — 不报错，
// PrepareConn 让 orgID 为 "" 走 fail-closed 路径。
//
// 调用方（prepareRLSConn）已确保 role=super_admin（SESSION=false），
// 可以直接查 organizations FORCE RLS。
func (r *slugUUIDResolver) Resolve(ctx context.Context, conn *pgx.Conn, slug string) (string, error) {
	if slug == "" {
		return "", nil
	}

	// 命中缓存
	r.mu.RLock()
	if e, ok := r.cache[slug]; ok && time.Now().Before(e.expires) {
		r.mu.RUnlock()
		if e.negative {
			return "", nil
		}
		return e.uuid, nil
	}
	r.mu.RUnlock()

	var uuid string
	err := conn.QueryRow(ctx,
		`SELECT id::text FROM guliangiam.organizations WHERE slug = $1 LIMIT 1`,
		slug,
	).Scan(&uuid)

	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		if err == pgx.ErrNoRows {
			r.cache[slug] = cacheEntry{negative: true, expires: time.Now().Add(r.ttl)}
			return "", nil
		}
		return "", err
	}
	r.cache[slug] = cacheEntry{uuid: uuid, expires: time.Now().Add(r.ttl)}
	return uuid, nil
}
