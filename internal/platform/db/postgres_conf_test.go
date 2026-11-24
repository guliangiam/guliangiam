package db

import (
	"testing"
	"time"
)

func TestPostgresConfigFromEnvUsesPostgresDSN(t *testing.T) {
	t.Setenv("DB_DSN", "")
	t.Setenv("POSTGRES_DSN", "postgres://postgres:postgres@127.0.0.1:5432/guliangiam?sslmode=disable")
	t.Setenv("DATABASE_URL", "")

	config, err := PostgresConfigFromEnv()
	if err != nil {
		t.Fatalf("PostgresConfigFromEnv() error = %v", err)
	}

	if config.DataSourceName != "postgres://postgres:postgres@127.0.0.1:5432/guliangiam?sslmode=disable" {
		t.Fatalf("DataSourceName = %q", config.DataSourceName)
	}
}

func TestPostgresConfigFromEnvUsesDatabaseURLFallback(t *testing.T) {
	t.Setenv("DB_DSN", "")
	t.Setenv("POSTGRES_DSN", "")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@127.0.0.1:5432/guliangiam_alt?sslmode=disable")

	config, err := PostgresConfigFromEnv()
	if err != nil {
		t.Fatalf("PostgresConfigFromEnv() error = %v", err)
	}

	if config.DataSourceName != "postgres://postgres:postgres@127.0.0.1:5432/guliangiam_alt?sslmode=disable" {
		t.Fatalf("DataSourceName = %q", config.DataSourceName)
	}
}

func TestPostgresConfigFromEnvPrefersDBDSN(t *testing.T) {
	const unified = "postgres://127.0.0.1:5432/unified?sslmode=disable"
	t.Setenv("DB_DSN", unified)
	t.Setenv("POSTGRES_DSN", "postgres://127.0.0.1:5432/legacy?sslmode=disable")
	t.Setenv("DATABASE_URL", "postgres://127.0.0.1:5432/alt?sslmode=disable")

	config, err := PostgresConfigFromEnv()
	if err != nil {
		t.Fatalf("PostgresConfigFromEnv() error = %v", err)
	}

	if config.DataSourceName != unified {
		t.Fatalf("DB_DSN should win, got %q", config.DataSourceName)
	}
}

// TestDataSourceNameKeyMatchesResolutionOrder 锁定启动日志所报告的 DSN 来源，
// 必须与实际解析顺序同源（DB_DSN > POSTGRES_DSN > DATABASE_URL），否则日志会
// 让人误判「连到哪个库」。
func TestDataSourceNameKeyMatchesResolutionOrder(t *testing.T) {
	cases := []struct {
		name    string
		dbDSN   string
		pgDSN   string
		url     string
		wantKey string
	}{
		{"all empty", "", "", "", ""},
		{"only DATABASE_URL", "", "", "postgres://alt/db", "DATABASE_URL"},
		{"only POSTGRES_DSN", "", "postgres://legacy/db", "", "POSTGRES_DSN"},
		{"only DB_DSN", "postgres://unified/db", "", "", "DB_DSN"},
		{"DB_DSN beats legacy", "postgres://unified/db", "postgres://legacy/db", "postgres://alt/db", "DB_DSN"},
		{"POSTGRES_DSN beats DATABASE_URL", "", "postgres://legacy/db", "postgres://alt/db", "POSTGRES_DSN"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DB_DSN", tc.dbDSN)
			t.Setenv("POSTGRES_DSN", tc.pgDSN)
			t.Setenv("DATABASE_URL", tc.url)

			if got := DataSourceNameKey(); got != tc.wantKey {
				t.Fatalf("DataSourceNameKey() = %q, want %q", got, tc.wantKey)
			}

			// 同一个解析器必须与 PostgresConfigFromEnv 选中的值一致
			if tc.wantKey == "" {
				if _, err := PostgresConfigFromEnv(); err == nil {
					t.Fatal("expected error when no DSN is configured")
				}
				return
			}
			cfg, err := PostgresConfigFromEnv()
			if err != nil {
				t.Fatalf("PostgresConfigFromEnv() error = %v", err)
			}
			wantDSN := map[string]string{"DB_DSN": tc.dbDSN, "POSTGRES_DSN": tc.pgDSN, "DATABASE_URL": tc.url}[tc.wantKey]
			if cfg.DataSourceName != wantDSN {
				t.Fatalf("DataSourceName = %q, want %q", cfg.DataSourceName, wantDSN)
			}
		})
	}
}

func TestPostgresConfigFromEnvGenericPoolOverrides(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://localhost/db")
	t.Setenv("POSTGRES_MAX_CONNS", "7")
	t.Setenv("POSTGRES_MIN_CONNS", "3")
	t.Setenv("DB_MAX_OPEN_CONNS", "20")
	t.Setenv("DB_MAX_IDLE_CONNS", "9")
	t.Setenv("DB_CONN_MAX_LIFETIME", "45m")

	config, err := PostgresConfigFromEnv()
	if err != nil {
		t.Fatalf("PostgresConfigFromEnv() error = %v", err)
	}

	if config.MaxConns != 20 {
		t.Fatalf("MaxConns = %d, want 20 (DB_MAX_OPEN_CONNS wins over POSTGRES_MAX_CONNS)", config.MaxConns)
	}
	if config.MinConns != 9 {
		t.Fatalf("MinConns = %d, want 9 (DB_MAX_IDLE_CONNS wins over POSTGRES_MIN_CONNS)", config.MinConns)
	}
	if config.MaxConnLifetime != 45*time.Minute {
		t.Fatalf("MaxConnLifetime = %v, want 45m", config.MaxConnLifetime)
	}
}

func TestPostgresConfigFromEnvInvalidLifetimeFallsBack(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://localhost/db")
	t.Setenv("DB_CONN_MAX_LIFETIME", "not-a-duration")

	config, err := PostgresConfigFromEnv()
	if err != nil {
		t.Fatalf("PostgresConfigFromEnv() error = %v", err)
	}

	if config.MaxConnLifetime != 30*time.Minute {
		t.Fatalf("MaxConnLifetime = %v, want 30m fallback", config.MaxConnLifetime)
	}
}
