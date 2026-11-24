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

// 端到端防线：种子 admin 登录真实路由后创建组织，必须一次成功，并且
// organizations / tenant_identities / tenant_instances / records / outbox
// 在同一事务里全部落库。历史上 POST /api/v1/organizations 曾因主 INSERT
// 参数列序错位（ParentID 落入 created_at 的 timestamptz 槽位）在首条写就
// 失败，导致整个请求 500 且不留审计与 outbox；本用例锁住完整写入链。
//
// 运行约定：
//
//	ALLOW_SCHEMA_RESET=1 \
//	ORG_CREATE_VERIFY_DSN=postgres://…/guliangiam_org_verify?sslmode=disable \
//	go test ./internal/bootstrap -run TestCreateOrganization_Integration -v
//
// 库名必须以 _verify 结尾，避免误指向开发库。
func TestCreateOrganization_Integration(t *testing.T) {
	dsn := os.Getenv("ORG_CREATE_VERIFY_DSN")
	if dsn == "" {
		t.Skip("[证据不足] ORG_CREATE_VERIFY_DSN is not set; isolated-database end-to-end assertion did not run")
	}
	if os.Getenv("ALLOW_SCHEMA_RESET") != "1" {
		t.Skip("[证据不足] ALLOW_SCHEMA_RESET=1 is required for the isolated schema reset")
	}

	parsed, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	if name := parsed.ConnConfig.Database; !strings.HasSuffix(name, "_verify") {
		t.Fatalf("ORG_CREATE_VERIFY_DSN must target a *_verify database, got %q", name)
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
	// 种子管理员带 need_update_password=true（AGENTS.md §3）：本用例验证的是
	// 组织创建写路径，先把强制改密闸门关掉，避免 403 掩盖真正的写失败。
	require.NoError(t, clearForcedPasswordChange(ctx, pool))

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

	status, body := postOrganization(t, client, server.URL, csrf, map[string]any{
		"slug":        "org-create-verify",
		"displayName": "Org Create Verify",
		"status":      "active",
		"languages":   []string{"en"},
	})
	require.Equal(t, http.StatusCreated, status, "create organization body: %s", body)

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &created))
	require.NotEmpty(t, created.ID)

	require.Equal(t, 1, countAsSuperAdmin(t, ctx, pool,
		`SELECT COUNT(*) FROM guliangiam.organizations WHERE id = $1::uuid`, created.ID))
	require.Equal(t, 1, countAsSuperAdmin(t, ctx, pool,
		`SELECT COUNT(*) FROM guliangiam.tenant_identities WHERE organization_id = $1::uuid`, created.ID))
	require.Equal(t, 1, countAsSuperAdmin(t, ctx, pool,
		`SELECT COUNT(*) FROM guliangiam.tenant_instances WHERE organization_id = $1::uuid AND is_primary`, created.ID))
	require.Equal(t, 1, countAsSuperAdmin(t, ctx, pool,
		`SELECT COUNT(*) FROM guliangiam.records WHERE organization_id = $1::uuid AND action = 'organization.created'`, created.ID))
	require.Equal(t, 1, countAsSuperAdmin(t, ctx, pool,
		`SELECT COUNT(*) FROM guliangiam.outbox WHERE organization_id = $1::uuid AND event_type = 'control.tenant_registered'`, created.ID))

	// 失败路径：同一 slug 再建一次必须被识别为 already_exists（409），
	// 事务回滚后不得留下第二条组织、租户或审计记录。
	status, body = postOrganization(t, client, server.URL, csrf, map[string]any{
		"slug":        "org-create-verify",
		"displayName": "Org Create Verify Duplicate",
		"status":      "active",
	})
	require.Equal(t, http.StatusConflict, status, "duplicate create body: %s", body)
	var failure struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &failure))
	require.Equal(t, "already_exists", failure.Error.Code)

	require.Equal(t, 1, countAsSuperAdmin(t, ctx, pool,
		`SELECT COUNT(*) FROM guliangiam.organizations WHERE slug = 'org-create-verify'`))
	require.Equal(t, 1, countAsSuperAdmin(t, ctx, pool,
		`SELECT COUNT(*) FROM guliangiam.tenant_identities WHERE organization_id = $1::uuid`, created.ID))
	require.Equal(t, 1, countAsSuperAdmin(t, ctx, pool,
		`SELECT COUNT(*) FROM guliangiam.records WHERE organization_id = $1::uuid`, created.ID))
}

func postOrganization(t *testing.T, client *http.Client, baseURL, csrf string, payload map[string]any) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/organizations", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

func countAsSuperAdmin(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, "SELECT set_config('app.current_role','super_admin',true)")
	require.NoError(t, err)
	var count int
	require.NoError(t, tx.QueryRow(ctx, query, args...).Scan(&count))
	return count
}

func clearForcedPasswordChange(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_role','super_admin',true)"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE guliangiam.user_credentials SET need_update_password = false
		WHERE user_id = (SELECT id FROM guliangiam.users WHERE username = 'admin')`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
