package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/db"
)

// 端到端防线：真种子 admin（all scope + approved_by NULL）登录真实路由，
// 必须看到非空组织列表。只有这一层能把“作用域解析 / 中间件提升 / RLS / 列表”
// 串成一条断言；其余单测都停在某一层内部。
//
// 运行约定（与 scripts/dev/validate_schema_replay.sh 同源）：
//
//	ALLOW_SCHEMA_RESET=1 \
//	P1_VERIFY_DSN=postgres://…/guliangiam_p1_verify?sslmode=disable \
//	go test ./internal/bootstrap -run TestSeedAdminSeesOrganizationList_Integration -v
//
// 库名必须以 _verify 结尾，避免误指向开发库。
func TestSeedAdminSeesOrganizationList_Integration(t *testing.T) {
	dsn := os.Getenv("P1_VERIFY_DSN")
	if dsn == "" {
		t.Skip("[证据不足] P1_VERIFY_DSN is not set; isolated-database end-to-end assertion did not run")
	}
	if os.Getenv("ALLOW_SCHEMA_RESET") != "1" {
		t.Skip("[证据不足] ALLOW_SCHEMA_RESET=1 is required for the isolated schema reset")
	}

	parsed, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	if name := parsed.ConnConfig.Database; !strings.HasSuffix(name, "_verify") {
		t.Fatalf("P1_VERIFY_DSN must target a *_verify database, got %q", name)
	}

	t.Setenv("POSTGRES_DSN", dsn)
	t.Setenv("DB_DSN", dsn)
	t.Setenv("APP_ENV", "development")
	t.Setenv("AUTO_MIGRATE", "true")
	t.Setenv("AUTO_SEED", "true")

	ctx := context.Background()
	cfg, err := db.PostgresConfigFromEnv()
	require.NoError(t, err)
	pool, err := db.NewPostgresPool(ctx, cfg)
	require.NoError(t, err)
	defer pool.Close()

	_, err = pool.Exec(ctx, "DROP SCHEMA IF EXISTS guliangiam CASCADE")
	require.NoError(t, err)
	require.NoError(t, migrateAndSeed(ctx, pool))

	// 种子只带 built-in 一个组织；补一条真实行，让“全量可见”与“只有内置”可区分。
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "SELECT set_config('app.current_role','super_admin',false)")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO guliangiam.organizations (id, slug, display_name, status)
		VALUES ('01a0ea76-0c00-7000-8000-0000000000aa'::uuid, 'p1-verify-second', 'P1 Verify Second', 'active')
		ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	runtime, err := NewRuntimeFromEnv(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	server := httptest.NewServer(NewChiRouter(runtime))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar, Timeout: 15 * time.Second}

	csrf := primeCSRFCookie(t, client, server.URL)
	loginAsSeedAdmin(t, client, server.URL, csrf)

	resp, err := client.Get(server.URL + "/api/v1/organizations?page=1&pageSize=10")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Items []struct {
			Slug string `json:"slug"`
		} `json:"items"`
		Page struct {
			Total int64 `json:"total"`
		} `json:"page"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.GreaterOrEqual(t, body.Page.Total, int64(2))
	require.GreaterOrEqual(t, len(body.Items), 2)

	var slugs []string
	for _, item := range body.Items {
		slugs = append(slugs, item.Slug)
	}
	require.Contains(t, slugs, "built-in")
}

func primeCSRFCookie(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()
	resp, err := client.Get(baseURL + "/api/v1/auth/contact")
	require.NoError(t, err)
	defer resp.Body.Close()
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "csrf_token" {
			return cookie.Value
		}
	}
	t.Fatal("auth contact response did not set csrf_token")
	return ""
}

func loginAsSeedAdmin(t *testing.T, client *http.Client, baseURL, csrf string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"applicationId":    "01a0ea76-0c00-7000-8000-000000000002",
		"organizationSlug": "built-in",
		"username":         "admin",
		"password":         "Guliang@2026", // 开发引导哨兵口令（AGENTS.md §3）
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/auth/login", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("admin login failed: %d %s", resp.StatusCode, string(raw))
	}
}
