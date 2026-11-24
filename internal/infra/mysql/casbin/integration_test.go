// integration_test.go — Casbin 策略存储的 MySQL 方言实现（真实 8.4 + 生成 DDL）。
// 仅当显式设置 MYSQL_DSN 时执行；默认 CI（无 MySQL）跳过。
package casbin

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	casbinv3 "github.com/casbin/casbin/v3"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	platformdb "github.com/guliangiam/guliangiam/internal/platform/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN not set; skipping MySQL casbin integration test")
	}
	normalized, err := platformdb.NormalizeMySQLDSN(dsn)
	require.NoError(t, err)
	db, err := sql.Open("mysql", normalized)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, db.PingContext(ctx), "MYSQL_DSN must point at a migrated MySQL 8.4+ database")
	return db
}

func insertIntegrationEnforcer(t *testing.T, db *sql.DB) (orgID, enforcerID string) {
	t.Helper()
	orgID = uuid.NewString()
	_, err := db.Exec(
		"INSERT INTO `organizations` (id, slug, display_name, status) VALUES (?, ?, ?, ?)",
		orgID, "it-casbin-"+orgID[:8], "Integration Org", "active",
	)
	require.NoError(t, err)

	enforcerID = uuid.NewString()
	_, err = db.Exec(
		"INSERT INTO `enforcers` (id, organization_id, name, display_name, is_enabled) VALUES (?, ?, ?, ?, 1)",
		enforcerID, orgID, "it-enforcer-"+enforcerID[:8], "Integration Enforcer",
	)
	require.NoError(t, err)
	return orgID, enforcerID
}

func cleanupIntegration(t *testing.T, db *sql.DB, orgID string) {
	t.Helper()
	if _, err := db.Exec("DELETE FROM `casbin_rules` WHERE enforcer_id IN (SELECT id FROM `enforcers` WHERE organization_id = ?)", orgID); err != nil {
		t.Logf("cleanup casbin_rules: %v", err)
	}
	if _, err := db.Exec("DELETE FROM `enforcers` WHERE organization_id = ?", orgID); err != nil {
		t.Logf("cleanup enforcers: %v", err)
	}
	if _, err := db.Exec("DELETE FROM `organizations` WHERE id = ?", orgID); err != nil {
		t.Logf("cleanup organizations: %v", err)
	}
}

func TestIntegration_PolicyStoreRoundTrip(t *testing.T) {
	db := openIntegrationDB(t)
	ctx := context.Background()

	orgID, enforcerID := insertIntegrationEnforcer(t, db)
	t.Cleanup(func() { cleanupIntegration(t, db, orgID) })

	store := Store{db: db, enforcerID: enforcerID}

	// SavePolicy 全量重写
	m := newModel(t)
	require.NoError(t, m.AddPolicy("p", "p", []string{"admin", "built-in", "*", "*"}))
	require.NoError(t, store.SavePolicy(m))

	loaded := newModel(t)
	require.NoError(t, store.LoadPolicy(loaded))
	require.Len(t, loaded["p"]["p"].Policy, 1)

	// persist.Adapter 兼容性：真实 Enforcer 能直接消费该 store。
	enforcer, err := casbinv3.NewEnforcer(loaded, store)
	require.NoError(t, err)
	ok, err := enforcer.Enforce("admin", "built-in", "users", "read")
	require.NoError(t, err)
	assert.True(t, ok, "管理员通配策略应放行")

	// 增量写入 / 精确删除
	require.NoError(t, store.AddPolicy("p", "p", []string{"alice", "built-in", "users", "read"}))
	loaded = newModel(t)
	require.NoError(t, store.LoadPolicy(loaded))
	require.Len(t, loaded["p"]["p"].Policy, 2)

	require.NoError(t, store.RemovePolicy("p", "p", []string{"alice", "built-in", "users", "read"}))
	loaded = newModel(t)
	require.NoError(t, store.LoadPolicy(loaded))
	require.Len(t, loaded["p"]["p"].Policy, 1)

	// 角色分组投影：先写两条，再收敛为一条（幂等）
	require.NoError(t, store.ReplaceUserGroupings(ctx, "alice", "built-in", []string{"admin", "viewer"}))
	loaded = newModel(t)
	require.NoError(t, store.LoadPolicy(loaded))
	require.Len(t, loaded["g"]["g"].Policy, 2)

	require.NoError(t, store.ReplaceUserGroupings(ctx, "alice", "built-in", []string{"admin"}))
	loaded = newModel(t)
	require.NoError(t, store.LoadPolicy(loaded))
	require.Len(t, loaded["g"]["g"].Policy, 1)

	// JIT 行：时间窗内放行、生命周期绑定、LoadPolicy 不可见
	now := time.Now().UTC()
	insertJITRow(t, db, enforcerID, "alice", "users", "read", now.Add(-time.Minute), now.Add(time.Hour), "lc-live")
	insertJITRow(t, db, enforcerID, "bob", "users", "read", now.Add(-time.Hour), now.Add(-time.Minute), "lc-expired")

	allowed, err := store.EnforceJIT(ctx, enforcerID, "lc-live", "alice", "built-in", "users", "read")
	require.NoError(t, err)
	assert.True(t, allowed, "窗口内的 JIT 策略应放行")

	allowed, err = store.EnforceJIT(ctx, enforcerID, "lc-expired", "bob", "built-in", "users", "read")
	require.NoError(t, err)
	assert.False(t, allowed, "已过期 JIT 策略不得放行")

	allowed, err = store.EnforceJIT(ctx, enforcerID, "lc-other", "alice", "built-in", "users", "read")
	require.NoError(t, err)
	assert.False(t, allowed, "生命周期不匹配时不得放行")

	loaded = newModel(t)
	require.NoError(t, store.LoadPolicy(loaded))
	assert.Len(t, loaded["p"]["p"].Policy, 1, "LoadPolicy 必须排除 JIT 行")

	// 解析 enforcer（MySQL 无 RLS，无需 GUC）
	resolved, err := store.ResolveEnforcerID(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, enforcerID, resolved)

	resolved, err = store.ResolveEnforcerUUID(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, enforcerID, resolved)
}

func insertJITRow(t *testing.T, db *sql.DB, enforcerID, subject, resource, action string, startsAt, expiresAt time.Time, lifecycleID string) {
	t.Helper()
	metadata := `{"source":"iga_jit","lifecycleId":"` + lifecycleID + `","startsAt":"` + startsAt.UTC().Format(time.RFC3339Nano) + `"}`
	_, err := db.Exec(
		"INSERT INTO `casbin_rules` (id, enforcer_id, ptype, v0, v1, v2, v3, v4, v5) VALUES (?, ?, 'p', ?, ?, ?, ?, ?, ?)",
		uuid.NewString(), enforcerID, subject, "built-in", resource, action,
		expiresAt.UTC().Format(time.RFC3339Nano), metadata,
	)
	require.NoError(t, err)
}
