package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// =============================================================================
// P0 批次 5 — DB
// =============================================================================

func DBdriver() string    { return strings.ToLower(strings.TrimSpace(os.Getenv("DB_DRIVER"))) }
func DBDSN() string       { return strings.TrimSpace(os.Getenv("DB_DSN")) }
func PostgresDSN() string { return strings.TrimSpace(os.Getenv("POSTGRES_DSN")) }
func DatabaseURL() string { return strings.TrimSpace(os.Getenv("DATABASE_URL")) }
func MySQLDSN() string    { return strings.TrimSpace(os.Getenv("MYSQL_DSN")) }
func MySQLURL() string    { return strings.TrimSpace(os.Getenv("MYSQL_URL")) }

const maxInt32 = 1<<31 - 1

// int32FromEnv 把已解析的环境整数值收窄到 int32；超界值钳位而不是回绕成负数
// （池大小被误写成超大数时，负数会让驱动直接失败或退化成 0）。
func int32FromEnv(n int) int32 {
	if n > maxInt32 {
		return maxInt32
	}
	return int32(n) //nolint:gosec // G115: n <= maxInt32 is enforced above
}

func PostgresPoolMax() int32 {
	if n, ok := parseIntKey("POSTGRES_MAX_CONNS"); ok && n > 0 {
		return int32FromEnv(n)
	}
	return 0
}
func PostgresPoolMin() int32 {
	if n, ok := parseIntKey("POSTGRES_MIN_CONNS"); ok && n >= 0 {
		return int32FromEnv(n)
	}
	return 0
}
func DBPoolMaxOpen() int {
	if n, ok := parseIntKey("DB_MAX_OPEN_CONNS"); ok && n > 0 {
		return n
	}
	// 方言兼容别名：MYSQL_MAX_OPEN_CONNS / POSTGRES_MAX_CONNS
	if n, ok := parseIntKey("MYSQL_MAX_OPEN_CONNS"); ok && n > 0 {
		return n
	}
	if n, ok := parseIntKey("POSTGRES_MAX_CONNS"); ok && n > 0 {
		return n
	}
	return 0
}
func DBPoolMaxIdle() int {
	if n, ok := parseIntKey("DB_MAX_IDLE_CONNS"); ok && n > 0 {
		return n
	}
	if n, ok := parseIntKey("MYSQL_MAX_IDLE_CONNS"); ok && n > 0 {
		return n
	}
	return 0
}

// truthyEnv 与 db.EnvTruthy 保持同一语义（true/1/yes/on），手册 3 对迁移/种子
// 开关约定的正是这四种写法。config 不能反向依赖 db（db 已依赖 config），
// 因此在本包内实现同一判定，避免同一环境变量在两层得到不同结论。
func truthyEnv(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

// AutoMigrate reports whether the process should run structure migrations on
// startup. Explicit AUTO_MIGRATE takes precedence; otherwise the default is off
// in every environment. Replaying the database on every boot made a local start
// depend on drift handling when a developer pointed .env at the shared
// development database (ADR-022/OPEN-API-34). The server exposes -migrate for
// the same opt-in, and production stays opt-in because the deploy pipeline owns
// migrations.
func AutoMigrate() bool {
	if _, set := os.LookupEnv("AUTO_MIGRATE"); set {
		return truthyEnv("AUTO_MIGRATE")
	}
	return false
}

// AutoSeed reports whether the process should run seed migrations on startup.
// Same opt-in contract as AutoMigrate: explicit env wins, otherwise off; the
// server's -seed flag is the other explicit trigger.
func AutoSeed() bool {
	if _, set := os.LookupEnv("AUTO_SEED"); set {
		return truthyEnv("AUTO_SEED")
	}
	return false
}

// BootstrapIntegrityStrict reports whether startup must refuse to serve when the
// bootstrap integrity check finds gaps (SEC-BOOT-INTEGRITY).
//
// Default is strict in every environment: unset or empty means true, so a
// database whose ledger/seed baseline is not proven complete can never serve.
// Only an explicit BOOTSTRAP_INTEGRITY_STRICT=false/0 disables the check, and
// that is honoured for non-production databases only — APP_ENV=production with
// the switch off is a configuration error and fails fast (same stance as the
// ALLOW_CHECKSUM_DRIFT production ban). An unparseable value is an error rather
// than a silent default, because a typo must not quietly weaken the baseline.
func BootstrapIntegrityStrict() (bool, error) {
	raw, set := os.LookupEnv("BOOTSTRAP_INTEGRITY_STRICT")
	if !set || strings.TrimSpace(raw) == "" {
		return true, nil
	}
	value, ok := parseBoolLiteral(raw)
	if !ok {
		return false, fmt.Errorf("BOOTSTRAP_INTEGRITY_STRICT must be true or false, got %q", raw)
	}
	if !value && IsProduction() {
		return false, fmt.Errorf("BOOTSTRAP_INTEGRITY_STRICT=false is forbidden when APP_ENV=production")
	}
	return value, nil
}

func SchemaContentHash() string { return os.Getenv("SCHEMA_CONTENT_HASH") }

// =============================================================================
// P0 批次 6 — Redis / Cache
// =============================================================================

func RedisAddr() string     { return strings.TrimSpace(os.Getenv("REDIS_ADDR")) }
func RedisPassword() string { return os.Getenv("REDIS_PASSWORD") }
func SessionCacheDriver() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("SESSION_CACHE_DRIVER")))
}

// Redis connection pool / circuit-breaker tuning — read by cache package.
// All return (value, ok); ok=false means "use code default", preserving the
// caller's zero-value-as-default contract (same semantic as the old envInt/envDuration helpers).
func RedisDB() int {
	if v, ok := parseIntKey("REDIS_DB"); ok {
		return v
	}
	return 0
}
func RedisMaxRetries() int {
	if v, ok := parseIntKey("REDIS_MAX_RETRIES"); ok {
		return v
	}
	return 3
}
func RedisPoolSize() int {
	if v, ok := parseIntKey("REDIS_POOL_SIZE"); ok {
		return v
	}
	return 0
}
func CircuitFailureThreshold() int {
	if v, ok := parseIntKey("CIRCUIT_FAILURE_THRESHOLD"); ok {
		return v
	}
	return 3
}
func CircuitRecoveryInterval() time.Duration {
	if v := strings.TrimSpace(os.Getenv("CIRCUIT_RECOVERY_INTERVAL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 5 * time.Second
}
func RedisDialTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv("REDIS_DIAL_TIMEOUT")); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 2 * time.Second
}
func RedisReadTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv("REDIS_READ_TIMEOUT")); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 2 * time.Second
}
func RedisWriteTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv("REDIS_WRITE_TIMEOUT")); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 2 * time.Second
}

// Per-IP rate-limiter tuning — read by middleware.
func RateLimitPerIPRequests() int {
	if v, ok := parseIntKey("RATE_LIMIT_PER_IP_REQUESTS"); ok {
		return v
	}
	return 10
}
func RateLimitPerIPWindowSeconds() int {
	if v, ok := parseIntKey("RATE_LIMIT_PER_IP_WINDOW"); ok {
		return v
	}
	return 60
}
func RateLimitPerIPCleanupIntervalSeconds() int {
	if v, ok := parseIntKey("RATE_LIMIT_PER_IP_CLEANUP_INTERVAL"); ok {
		return v
	}
	return 300
}

// DB connection max-lifetime — read by db/mysql (postgres uses pool-level).
func DBConnMaxLifetime() time.Duration {
	if v := strings.TrimSpace(os.Getenv("DB_CONN_MAX_LIFETIME")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 30 * time.Minute
}
