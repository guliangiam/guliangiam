// wire_db.go — DB Driver selection + Dialect resolution + UoW 构造。
// 从 runtime.go 的 NewRuntime 中抽离，保持 bootstrap 包级 private function。
package bootstrap

import (
	"database/sql"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/guliangiam/guliangiam/internal/infra/cache"
	"github.com/guliangiam/guliangiam/internal/infra/dialect"
	mysqluow "github.com/guliangiam/guliangiam/internal/infra/mysql/uow"
	infrapg "github.com/guliangiam/guliangiam/internal/infra/postgres"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
)

// wireDeps 返回 (unitOfWork, error) — NewRuntime 核心依赖。
// sessionCache 跨 driver 共享（postgresuow 内部 SetSessionCache 不区分调用方）。
func wireDeps(pool *pgxpool.Pool, mysqlDB *sql.DB, driver, dialectHint string, sessionCache cache.SessionCache) (appuow.UnitOfWork, error) {
	// ── DB Driver selection ────────────────────────────────────────────────────
	// DB_DRIVER env 选择存储后端。默认 "pgx"（PostgreSQL via pgxpool）。
	// "mysql" 使用 MySQL 8.4+ (go-sql-driver/mysql) — 目前仅 ReBAC 模块支持，
	// 其他模块会在构造 unitOfWork 时 panic（严格失败，不静默降级）。
	//
	// 未来：逐个模块补 MySQL 存储层 → mysqluow.New(mysqlDB) 完整替代。
	slog.Info("db driver selected",
		"requested", config.DBdriver(),
		"active", driver,
	)

	// ── Dialect resolution ────────────────────────────────────────────────
	// driver 值（pgx / mysql）与 dialect registry key（postgres / mysql）
	// 不完全一致，这里做一层显式映射。dialect.New 对未知 key 返回 error，
	// runtime 的 switch 已在上游 fail-fast，此处仅兜底。
	// dialectKey 映射基于 dialectHint（原始 DB_DRIVER env 值）。
	// 这是唯一真正区分方言的信号——driver 只是 wire protocol。
	dialectKey := driverPostgres // default
	switch dialectHint {
	case driverPgx, driverPostgres, "postgresql":
		dialectKey = driverPostgres
	case driverMySQL, "mariadb":
		dialectKey = driverMySQL
	case driverOceanBase, "oceanbase-mysql":
		dialectKey = driverOceanBase
	case driverTiDB:
		dialectKey = driverTiDB
	case driverPolarDB, "polardb-mysql":
		dialectKey = driverPolarDB
	case driverKingbase, "kingbasees":
		dialectKey = driverKingbase
	case driverGaussDB, "opengauss":
		dialectKey = driverGaussDB
	}
	d, err := dialect.New(dialectKey)
	if err != nil {
		return nil, apperrors.NewUnavailable("dialect resolution failed: " + err.Error())
	}
	slog.Info("dialect resolved",
		"name", d.Name(),
		"returning", d.Supports(dialect.FeatureReturning),
		"upsert", d.Supports(dialect.FeatureUpsert),
		"skip_locked", d.Supports(dialect.FeatureSkipLocked),
		"rls", d.Supports(dialect.FeatureRLS),
		"json_type", d.Supports(dialect.FeatureJSONType),
	)

	// ── UnitOfWork 双基线构造 ────────────────────────────────────────────
	// driver 决定 UoW 实现：pgx 用 postgresuow（共享 pgxpool 连接），
	// mysql 协议 tier 1（MySQL/OceanBase/TiDB/PolarDB）用 mysqluow（共享 *sql.DB）。
	// PG 协议 tier 2（KingbaseES/GaussDB）走 postgresuow。两个实现都满足 appuow.UnitOfWork。
	var unitOfWork appuow.UnitOfWork
	switch driver {
	case driverMySQL:
		// mysqluow 共享 *sql.DB — OceanBase/TiDB/PolarDB wire 兼容
		unitOfWork = mysqluow.New(mysqlDB)
		postgresuow.SetSessionCache(sessionCache) // session cache 跨 driver 共享
	default:
		// pgx / kingbase / gaussdb 都走 pgx 驱动 + postgresuow
		// pool 作为 variadic 第二参传给 UOW — postgres organizations.Repository
		// 需要 pool.Acquire + set_config 绕过 FORCE RLS（public by-slug endpoint）。
		unitOfWork = postgresuow.New(infrapg.NewInstrumentedDBTX(pool), pool)
		postgresuow.SetSessionCache(sessionCache)
	}
	return unitOfWork, nil
}
