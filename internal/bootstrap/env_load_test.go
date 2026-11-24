package bootstrap

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/db"
)

func writeDotenvFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestInitEnvProductionSkipsDotEnv 锁定 F5 的回归面：生产环境不得从工作目录的
// .env 读取任何变量，DSN 必须来自编排层注入的变量。
//
// 回归场景（修复前真实发生过）：只注入 DATABASE_URL，而工作目录残留 .env（内含
// POSTGRES_DSN）→ godotenv 把 POSTGRES_DSN 补进环境 → 解析优先级
// DB_DSN > POSTGRES_DSN > DATABASE_URL 让进程静默连到 .env 指向的库。
func TestInitEnvProductionSkipsDotEnv(t *testing.T) {
	dir := t.TempDir()
	writeDotenvFile(t, dir, ".env", "POSTGRES_DSN=postgres://dotenv-leak:leak@localhost:5432/wrong-db\n")

	t.Setenv("APP_ENV", "production")
	unsetEnv(t, "DB_DSN")
	unsetEnv(t, "POSTGRES_DSN")
	t.Setenv("DATABASE_URL", "postgres://orchestrator:secret@db:5432/guliangiam?sslmode=disable")
	t.Chdir(dir)

	loaded := initEnv()
	if len(loaded) != 0 {
		t.Fatalf("production must not load dotenv files, got %v", loaded)
	}
	if got := os.Getenv("POSTGRES_DSN"); got != "" {
		t.Fatalf("dotenv POSTGRES_DSN leaked into production environment: %q", got)
	}
	if key := db.DataSourceNameKey(); key != "DATABASE_URL" {
		t.Fatalf("DSN source = %q, want DATABASE_URL (orchestrator-injected)", key)
	}
}

// TestInitEnvDevelopmentLoadsDotEnv 保证非生产环境的便利性没有被 F5 修复误伤。
func TestInitEnvDevelopmentLoadsDotEnv(t *testing.T) {
	dir := t.TempDir()
	writeDotenvFile(t, dir, ".env.local", "SESSION_HMAC_KEY=from-local-file\n")
	writeDotenvFile(t, dir, ".env", "POSTGRES_DSN=postgres://dotenv:dotenv@localhost:5432/dev-db\n")

	t.Setenv("APP_ENV", "development")
	unsetEnv(t, "DB_DSN")
	unsetEnv(t, "POSTGRES_DSN")
	unsetEnv(t, "DATABASE_URL")
	unsetEnv(t, "SESSION_HMAC_KEY")
	t.Chdir(dir)

	loaded := initEnv()
	if len(loaded) != 2 || loaded[0] != ".env.local" || loaded[1] != ".env" {
		t.Fatalf("development must load .env.local then .env, got %v", loaded)
	}
	if got := os.Getenv("POSTGRES_DSN"); got != "postgres://dotenv:dotenv@localhost:5432/dev-db" {
		t.Fatalf("POSTGRES_DSN = %q, want the value from .env", got)
	}
	if got := os.Getenv("SESSION_HMAC_KEY"); got != "from-local-file" {
		t.Fatalf("SESSION_HMAC_KEY = %q, want the value from .env.local", got)
	}
}

// TestInitEnvInjectedValueWinsOverDotEnv 记录 godotenv 的非覆盖语义：
// 显式注入的变量优先于同名 .env 条目，便于部署侧用同名键覆盖。
func TestInitEnvInjectedValueWinsOverDotEnv(t *testing.T) {
	dir := t.TempDir()
	writeDotenvFile(t, dir, ".env", "POSTGRES_DSN=postgres://dotenv:dotenv@localhost:5432/dev-db\n")

	t.Setenv("APP_ENV", "development")
	unsetEnv(t, "DB_DSN")
	t.Setenv("POSTGRES_DSN", "postgres://injected:wins@localhost:5432/target-db")
	t.Chdir(dir)

	initEnv()
	if got := os.Getenv("POSTGRES_DSN"); got != "postgres://injected:wins@localhost:5432/target-db" {
		t.Fatalf("explicitly injected POSTGRES_DSN must win over .env, got %q", got)
	}
}

// TestInitEnvMissingFilesAreTolerated 没有 .env 时不应报错或返回伪造条目。
func TestInitEnvMissingFilesAreTolerated(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Chdir(t.TempDir())

	if loaded := initEnv(); len(loaded) != 0 {
		t.Fatalf("no dotenv files exist, expected none loaded, got %v", loaded)
	}
}

// captureBootstrapLog 把启动期日志写入内存，便于断言「打印了来源、没打印凭据」。
func captureBootstrapLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// TestLogEnvironmentBootstrapReportsDSNSourceWithoutValue 锁定 X3 的日志契约：
// 启动日志必须给出最终生效的 DSN 来源键名，且永不包含连接串、口令或库名。
func TestLogEnvironmentBootstrapReportsDSNSourceWithoutValue(t *testing.T) {
	const (
		dsn      = "postgres://leak_user:leak_secret@localhost:5432/secret_db?sslmode=disable"
		database = "secret_db"
	)
	t.Setenv("DB_DSN", "")
	t.Setenv("POSTGRES_DSN", dsn)
	t.Setenv("DATABASE_URL", "")

	buf := captureBootstrapLog(t)
	logEnvironmentBootstrap(nil)

	out := buf.String()
	if !strings.Contains(out, `"postgres DSN source"`) {
		t.Fatalf("missing postgres DSN source log: %s", out)
	}
	if !strings.Contains(out, `"env_key":"POSTGRES_DSN"`) {
		t.Fatalf("env_key should report POSTGRES_DSN, got: %s", out)
	}
	if !strings.Contains(out, `"precedence":"DB_DSN > POSTGRES_DSN > DATABASE_URL"`) {
		t.Fatalf("log should state precedence, got: %s", out)
	}
	for _, secret := range []string{dsn, "leak_secret", database} {
		if strings.Contains(out, secret) {
			t.Fatalf("startup log leaked DSN material %q: %s", secret, out)
		}
	}
}

// TestLogEnvironmentBootstrapReportsUnsetDSN 无 DSN 时也必须打印来源（unset），
// 让「没配」与「日志缺失」可区分。
func TestLogEnvironmentBootstrapReportsUnsetDSN(t *testing.T) {
	unsetEnv(t, "DB_DSN")
	unsetEnv(t, "POSTGRES_DSN")
	unsetEnv(t, "DATABASE_URL")

	buf := captureBootstrapLog(t)
	logEnvironmentBootstrap(nil)

	if out := buf.String(); !strings.Contains(out, `"env_key":"unset"`) {
		t.Fatalf("expected env_key=unset when no DSN configured, got: %s", out)
	}
}

// TestLogEnvironmentBootstrapWarnsOnDotenvDSNConflict 覆盖 OPEN-API-45 的真实
// 故障路径：编排层只注入 DATABASE_URL，工作目录残留 .env（内含 POSTGRES_DSN）。
// dotenv 会补入 POSTGRES_DSN，按优先级静默压过 DATABASE_URL；启动期必须 WARN，
// 且 WARN 只含键名/来源类别/优先级，不含连接串、口令或库名。
func TestLogEnvironmentBootstrapWarnsOnDotenvDSNConflict(t *testing.T) {
	const (
		dotenvDSN     = "postgres://dotenv_user:dotenv_secret@localhost:5432/wrong_db"
		injectedDSN   = "postgres://orchestrator:inject_secret@db:5432/guliangiam"
		wrongDatabase = "wrong_db"
	)

	dir := t.TempDir()
	writeDotenvFile(t, dir, ".env", "POSTGRES_DSN="+dotenvDSN+"\n")

	t.Setenv("APP_ENV", "development")
	unsetEnv(t, "DB_DSN")
	unsetEnv(t, "POSTGRES_DSN")
	t.Setenv("DATABASE_URL", injectedDSN)
	t.Chdir(dir)

	loaded := loadDotenv()
	if got := os.Getenv("POSTGRES_DSN"); got != dotenvDSN {
		t.Fatalf("dotenv must introduce POSTGRES_DSN, got %q", got)
	}
	if !loaded.Introduced["POSTGRES_DSN"] {
		t.Fatalf("POSTGRES_DSN should be recorded as dotenv-introduced, got %v", loaded.Introduced)
	}

	buf := captureBootstrapLog(t)
	logEnvironmentBootstrapWithOrigin(loaded.Files, loaded.Introduced)

	out := buf.String()
	if !strings.Contains(out, `"multiple postgres DSN sources are configured`) {
		t.Fatalf("missing conflict WARN: %s", out)
	}
	if !strings.Contains(out, `"sources":"POSTGRES_DSN(dotenv),DATABASE_URL(env)"`) {
		t.Fatalf("conflict WARN should list keys and origins, got: %s", out)
	}
	if !strings.Contains(out, `"env_key":"POSTGRES_DSN"`) {
		t.Fatalf("conflict WARN should name the winning key, got: %s", out)
	}
	if !strings.Contains(out, `"precedence":"DB_DSN > POSTGRES_DSN > DATABASE_URL"`) {
		t.Fatalf("conflict WARN should state precedence, got: %s", out)
	}
	for _, secret := range []string{dotenvDSN, injectedDSN, "dotenv_secret", "inject_secret", wrongDatabase} {
		if strings.Contains(out, secret) {
			t.Fatalf("conflict WARN leaked DSN material %q: %s", secret, out)
		}
	}
}

// TestLogDataSourceConflictsSilentForSingleSource 保证单来源时不会噪声告警。
func TestLogDataSourceConflictsSilentForSingleSource(t *testing.T) {
	unsetEnv(t, "DB_DSN")
	t.Setenv("POSTGRES_DSN", "postgres://only:only@localhost:5432/only")
	unsetEnv(t, "DATABASE_URL")

	buf := captureBootstrapLog(t)
	logDataSourceConflicts("POSTGRES_DSN", nil)

	if out := buf.String(); strings.Contains(out, `"multiple postgres DSN sources are configured`) {
		t.Fatalf("single source must not warn, got: %s", out)
	}
}
