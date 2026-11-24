package bootstrap

import (
	"context"
	"log/slog"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	mysqlusers "github.com/guliangiam/guliangiam/internal/infra/mysql/users"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	"github.com/guliangiam/guliangiam/internal/platform/db"
)

func driverNormalize(env string) (normalized, wireProtocol string) {
	if env == "" {
		env = driverPgx
	}
	// driver → internal normalized name, and → wire protocol for Dialect resolution
	switch env {
	case driverPgx, driverPostgres, "postgresql",
		driverKingbase, "kingbasees",
		driverGaussDB, "opengauss":
		return driverPgx, env // 保留 kingbase/gaussdb 名称给 dialectKey 映射
	case driverMySQL, "mariadb",
		driverOceanBase, "oceanbase-mysql",
		driverTiDB,
		driverPolarDB, "polardb-mysql":
		return driverMySQL, env
	default:
		return "", env // 未知 — 由调用方 fail-fast
	}
}

func NewRuntimeFromEnv(ctx context.Context) (*Runtime, error) {
	// DB_DRIVER env 决定存储后端。默认 "pgx"（严格失败——无静默降级）。
	driver, wire := driverNormalize(config.DBdriver())
	if driver == "" {
		raw := config.DBdriver()
		if raw == "" {
			raw = driverPgx
		}
		return nil, apperrors.NewUnavailable("unknown DB_DRIVER: " + raw +
			" — supported: pgx/postgres, mysql, oceanbase, tidb, polardb, kingbase, gaussdb")
	}

	// wire 传给 newRuntimePostgres/newRuntimeMySQL 内部做 dialectKey 映射
	switch driver {
	case driverMySQL:
		return newRuntimeMySQLWithDialect(ctx)
	case driverPgx:
		return newRuntimePostgresWithDialect(ctx, wire)
	default:
		return nil, apperrors.NewUnavailable("internal driver error: " + driver)
	}
}

func newRuntimePostgresWithDialect(ctx context.Context, dialectHint string) (*Runtime, error) {
	cfg, err := db.PostgresConfigFromEnv()
	if err != nil {
		return nil, err
	}

	pool, err := db.NewPostgresPool(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// 启动自检：RLS 依赖非 superuser / 非 BYPASSRLS 连接角色；不合规直接 fail-fast。
	// 仅 PostgreSQL 方言执行；MySQL 路径不适用（见 ADR-023 / OPEN-API-37）。
	if err := verifyPostgresRoleIsolation(ctx, queryPostgresRoleAttributes(pool)); err != nil {
		pool.Close()
		return nil, err
	}

	runtime, err := NewRuntime(ctx, pool, nil, driverPgx, dialectHint)
	if err != nil {
		pool.Close()
		return nil, err
	}
	// Payment connectivity is probed against the vendor API; the provider
	// service must not report success from config-only validation.
	wireProviderConnectivityProbes(runtime)
	return runtime, nil
}

func newRuntimeMySQLWithDialect(ctx context.Context) (*Runtime, error) {
	cfg, err := db.MySQLConfigFromEnv()
	if err != nil {
		return nil, err
	}

	slog.Info("connecting mysql driver",
		"dsn", maskMySQLDSN(cfg.DataSourceName),
		"max_open_conns", cfg.MaxOpenConns,
	)

	mysqlDB, err := db.NewMySQLPool(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// 启动期 schema/仓储一致性探针：MySQL 镜像仓储的 SQL 必须与
	// contracts/schema 生成的 DDL 同构。列漂移在编译期不可见，首个请求
	// 才会以 Unknown column → 500 暴露；这里把它前移为可控的启动失败，
	// 而不是让进程带着不可用的身份面启动（OPEN-API-37 第一阶段已完成 users）。
	if err := mysqlusers.Preflight(ctx, mysqlDB); err != nil {
		_ = mysqlDB.Close()
		return nil, apperrors.NewUnavailable(
			"DB_DRIVER=mysql is not a supported runtime: the MySQL users repository does not match the " +
				"generated schema (db/mysql/V1 creates normalized user_* tables). Use PostgreSQL. " +
				"Registry: OPEN-API-37. underlying error: " + err.Error())
	}

	runtime, err := NewRuntime(ctx, nil, mysqlDB, driverMySQL, driverMySQL)
	if err != nil {
		_ = mysqlDB.Close()
		return nil, err
	}
	return runtime, nil
}

// maskMySQLDSN 把 MySQL DSN 里的密码替换成 ****，避免日志泄漏凭据。
// DSN 格式：user:pass@tcp(host:port)/dbname
func maskMySQLDSN(dsn string) string {
	atIdx := 0
	for i := 0; i < len(dsn); i++ {
		if dsn[i] == '@' {
			atIdx = i
			break
		}
	}
	if atIdx == 0 {
		// 无密码部分或没有 @，直接返回原样（不泄漏）
		return dsn
	}
	// user:pass → user:****
	colonIdx := 0
	for i := 0; i < atIdx; i++ {
		if dsn[i] == ':' {
			colonIdx = i
			break
		}
	}
	if colonIdx > 0 {
		return dsn[:colonIdx+1] + "****" + dsn[atIdx:]
	}
	return dsn
}

func (r *Runtime) Close() {
	if r == nil {
		return
	}

	if r.Lifecycle.SessionCleanupStopCh != nil {
		close(r.Lifecycle.SessionCleanupStopCh)
	}

	if r.Handlers.RADIUSHandler != nil {
		if err := r.Handlers.RADIUSHandler.StopServer(); err != nil {
			slog.Warn("radius server shutdown failed", "error", err)
		}
	}

	if r.Services.LdapSyncService != nil {
		r.Services.LdapSyncService.StopScheduler()
	}

	if r.Services.VerificationService.RateLimiter != nil {
		r.Services.VerificationService.RateLimiter.Stop()
	}

	if r.Workers.OutboxWorker != nil {
		r.Workers.OutboxWorker.Stop()
	}

	if r.Workers.NotificationWorker != nil {
		r.Workers.NotificationWorker.Stop()
	}

	if r.Workers.AuditChain != nil {
		r.Workers.AuditChain.Stop()
	}

	// ctx-based workers: MeteringAggregate (AIGatewayProbe has its own Stop())
	if r.Lifecycle.WorkersCancel != nil {
		r.Lifecycle.WorkersCancel()
	}

	if r.Lifecycle.ShutdownTelemetry != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = r.Lifecycle.ShutdownTelemetry(ctx)
	}

	if r.Lifecycle.SessionCacheCloser != nil {
		if err := r.Lifecycle.SessionCacheCloser.Close(); err != nil {
			slog.Warn("session cache close failed", "error", err)
		}
	}
	if r.Lifecycle.FederationJWKSCache != nil {
		r.Lifecycle.FederationJWKSCache.Close()
	}

	if r.DB.Pool != nil {
		r.DB.Pool.Close()
	}
	if r.DB.MySQLPool != nil {
		_ = r.DB.MySQLPool.Close()
	}
}

// envOrEmpty 是 bootstrap 层 **可选第三方集成** 的统一入口。
//
// 设计决策（ADR-028）：env.go 注册核心业务配置（DB/Session/RateLimit/Messaging 核心），
// 而可选的第三方集成（STORAGE_* 多后端、SMS 多 Provider、CAPTCHA 多后端、Notification
// 多渠道、Identity Verification 多后端等）共 80+ 变量统一走本函数，避免 env.go 膨胀
// 成 600 行的巨型注册表。SMTP/SENDGRID/RESEND 三大邮件渠道已提升为 config getter（
// SMTPHost/SendGridAPIKey/ResendAPIKey 等），其他可选 Provider 保持 envOrEmpty 读取。
