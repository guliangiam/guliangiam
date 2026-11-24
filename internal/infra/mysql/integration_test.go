package mysql_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	domain "github.com/guliangiam/guliangiam/internal/domain/b2b"
	b2bmysql "github.com/guliangiam/guliangiam/internal/infra/mysql/b2b"
	platformdb "github.com/guliangiam/guliangiam/internal/platform/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 真实 MySQL 运行期门控：只有显式提供 MYSQL_DSN 时才连接真实库。
// DSN 示例：root:root@tcp(127.0.0.1:3306)/guliangiam_audit_mysql_1028
func openMySQL(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN not set; skipping MySQL integration test")
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

// TestMySQLSchemaAndB2BRuntime 覆盖三件事：
//  1. 生成的 MySQL DDL 能建出本仓储依赖的表（schema 与 PG 基线同构）；
//  2. b2b 仓储在真实 MySQL 上完成 insert / read / 唯一冲突 / 跨租户隔离；
//  3. 终态行不能被 Update 改回 pending（终态不可逆由 SQL 兜底）。
func TestMySQLSchemaAndB2BRuntime(t *testing.T) {
	db := openMySQL(t)
	ctx := context.Background()

	var tables int
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()").
		Scan(&tables))
	require.GreaterOrEqual(t, tables, 140, "generated MySQL schema must create every table")

	suffix := fmt.Sprint(time.Now().UnixNano())
	orgA := "it-org-a-" + suffix
	orgB := "it-org-b-" + suffix
	require.NoError(t, insertOrganization(ctx, db, orgA))
	require.NoError(t, insertOrganization(ctx, db, orgB))
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM organizations WHERE id IN (?, ?)", orgA, orgB)
	})

	repo := b2bmysql.NewRepository(db)
	now := time.Now().UTC().Truncate(time.Second)
	domainA := &domain.OrganizationDomain{
		ID:                "it-domain-a-" + suffix,
		OrganizationID:    orgA,
		Domain:            "example-" + suffix + ".test",
		VerificationToken: "token-a",
		Status:            "pending",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	require.NoError(t, repo.Create(ctx, domainA))

	got, err := repo.GetByDomain(ctx, orgA, domainA.Domain)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "pending", got.Status)

	// 重复认领必须被 uq_org_domain 拒绝。
	duplicate := *domainA
	duplicate.ID = "it-domain-dup-" + suffix
	require.Error(t, repo.Create(ctx, &duplicate), "uq_org_domain must reject a duplicate claim")

	// 终态不可逆：先转 verified，再尝试 Update 回 pending 必须失败。
	domainA.Status = "verified"
	domainA.VerifiedAt = now
	domainA.UpdatedAt = now
	require.NoError(t, repo.Update(ctx, domainA))

	seen, err := repo.FindVerifiedByDomain(ctx, domainA.Domain)
	require.NoError(t, err)
	require.NotNil(t, seen)
	assert.Equal(t, "verified", seen.Status)

	revert := *seen
	revert.Status = "pending"
	revert.VerifiedAt = time.Time{}
	require.Error(t, repo.Update(ctx, &revert), "terminal rows must not be moved back to pending")

	afterRevert, err := repo.GetByID(ctx, domainA.ID)
	require.NoError(t, err)
	assert.Equal(t, "verified", afterRevert.Status)

	// 跨租户隔离：orgA 的 List 只返回自己的域名。
	items, total, err := repo.List(ctx, domain.ListQuery{OrganizationID: orgA, Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, orgA, items[0].OrganizationID)

	// 另一个租户看不到 orgA 的域名。
	otherItems, otherTotal, err := repo.List(ctx, domain.ListQuery{OrganizationID: orgB, Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Zero(t, otherTotal)
	assert.Empty(t, otherItems)
}

func insertOrganization(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx,
		"INSERT INTO organizations (id, slug, display_name, status) VALUES (?, ?, ?, 'active')",
		id, id, id)
	return err
}
