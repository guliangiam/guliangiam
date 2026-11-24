package db

import (
	"log/slog"
	"runtime"
	"strconv"

	"time"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func calcPoolConns() (maxConns, minConns int32) {
	// 先在 int 空间夹紧到 [10,50] 再收窄，转换不可能溢出；顺序反过来会让
	// 超大 NumCPU 的结果在 int32 上回绕成负数。
	n := runtime.NumCPU()*2 + 1
	if n > 50 {
		n = 50
	}
	if n < 10 {
		n = 10
	}
	maxConns = int32(n) //nolint:gosec // G115: clamped to [10,50] above

	minConns = maxConns / 4
	if minConns < 2 {
		minConns = 2
	}
	return
}

func applyPoolEnvOverrides(maxConns, minConns int32) (maxOut, minOut int32) {
	if v := strconv.Itoa(int(config.PostgresPoolMax())); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil && n > 0 {
			maxConns = int32(n)
		}
	}
	if v := strconv.Itoa(int(config.PostgresPoolMin())); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil && n > 0 {
			minConns = int32(n)
		}
	}
	return maxConns, minConns
}

// applyGenericPoolEnvOverrides applies the dialect-agnostic DB_* pool overrides.
// They run after the legacy POSTGRES_* aliases so the unified names win (契约 /
// 环境变量注册表 D3_db_driver).
func applyGenericPoolEnvOverrides(maxConns, minConns int32) (maxOut, minOut int32) {
	maxOut, minOut = maxConns, minConns
	if v := strconv.Itoa(config.DBPoolMaxOpen()); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil && n > 0 {
			maxOut = int32(n)
		}
	}
	if v := strconv.Itoa(config.DBPoolMaxIdle()); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil && n > 0 {
			minOut = int32(n)
		}
	}
	return maxOut, minOut
}

// dataSourceNameSources 按优先级列出 PostgreSQL DSN 的来源环境变量。
// 新增/调整来源时只改这里 —— PostgresConfigFromEnv 与 DataSourceNameKey 共用
// resolveDataSourceName，避免「解析顺序」和「日志报告的顺序」各写一份而漂移。
func dataSourceNameSources() []struct{ Key, Value string } {
	return []struct{ Key, Value string }{
		{"DB_DSN", config.DBDSN()},
		{"POSTGRES_DSN", config.PostgresDSN()},
		{"DATABASE_URL", config.DatabaseURL()},
	}
}

// resolveDataSourceName 返回 (来源键名, DSN 值)；两者都为空表示未配置。
func resolveDataSourceName() (key, dsn string) {
	for _, s := range dataSourceNameSources() {
		if s.Value != "" {
			return s.Key, s.Value
		}
	}
	return "", ""
}

// DataSourceNameKey 只返回当前生效的 DSN 来源键名，从不返回值本身。
// 启动日志用它回答「这次到底连的是哪个变量给的库」——.env 与编排层注入互相
// 压盖时（例如只注入 DATABASE_URL 而同目录 .env 里有 POSTGRES_DSN），
// 没有这条日志就只能靠猜。
func DataSourceNameKey() string {
	key, _ := resolveDataSourceName()
	return key
}

// PostgresDataSourceName returns the resolved PostgreSQL DSN value with the
// same precedence as PostgresConfigFromEnv (DB_DSN > POSTGRES_DSN > DATABASE_URL).
// Empty means none of the keys is configured. The value is a credential, so
// callers must pass it straight to the driver and never log it.
func PostgresDataSourceName() string {
	_, dsn := resolveDataSourceName()
	return dsn
}

func PostgresConfigFromEnv() (PostgresConfig, error) {
	_, dataSourceName := resolveDataSourceName()

	if dataSourceName == "" {
		return PostgresConfig{}, apperrors.NewUnavailable("postgres DSN not configured: set DB_DSN, POSTGRES_DSN or DATABASE_URL")
	}

	maxConns, minConns := calcPoolConns()
	maxConns, minConns = applyPoolEnvOverrides(maxConns, minConns)
	maxConns, minConns = applyGenericPoolEnvOverrides(maxConns, minConns)

	cfg := PostgresConfig{
		DataSourceName:    dataSourceName,
		MaxConns:          maxConns,
		MinConns:          minConns,
		MaxConnLifetime:   config.DBConnMaxLifetime(),
		MaxConnIdleTime:   10 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
	}

	slog.Info("postgres pool config",
		"max_conns", cfg.MaxConns,
		"min_conns", cfg.MinConns,
		"max_conn_lifetime", cfg.MaxConnLifetime,
		"max_conn_idle_time", cfg.MaxConnIdleTime,
		"health_check_period", cfg.HealthCheckPeriod,
	)

	return cfg, nil
}
