//go:build integration

package scopeauthorizer

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infrapg "github.com/guliangiam/guliangiam/internal/infra/postgres"
	auditinfra "github.com/guliangiam/guliangiam/internal/infra/postgres/authorization_audit"
)

// TestAuditWriter_EndToEnd 真实 DB 集成测试：
// 1. 构造 AuditWriter
// 2. Append 3 条审计记录
// 3. 读回 authorization_audit_records + authorization_chain_state
// 4. 验证每条记录 hash 正确 + prev_hash 链式衔接
// 5. 验证 chain_state.last_hash = 最后一条 hash
//
// 前置条件：V5 + V6 DDL 已跑，env DATABASE_URL 设置。
// 运行：go test -tags=integration -run TestAuditWriter_EndToEnd -v
func TestAuditWriter_EndToEnd(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skip integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "connect to postgres")
	defer pool.Close()

	// 清理之前的测试数据 + 重置 chain_state
	// set_tenant_context 是 transaction-level，必须在同一个 tx 里做
	cleanupTx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = cleanupTx.Rollback(ctx) }()
	if _, err := cleanupTx.Exec(ctx, `SELECT guliangiam.set_tenant_context(NULL::uuid, 'super_admin')`); err != nil {
		t.Fatal(err)
	}
	if _, err := cleanupTx.Exec(ctx, `TRUNCATE guliangiam.authorization_audit_records CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := cleanupTx.Exec(ctx, `UPDATE guliangiam.authorization_chain_state SET last_hash = NULL WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	require.NoError(t, cleanupTx.Commit(ctx))

	log := slog.Default()
	// The writer now takes the audit repository (hash-chain transaction lives
	// there), not the raw pool.
	writer := NewAuditWriter(auditinfra.NewRepository(pool), log)

	actorID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	srcTenant := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	tgtTenant := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	// ── 写 3 条审计 ──────────────────────────────────────────────
	for i, tc := range []struct {
		rec    AuditRecord
		expect string
	}{
		{ // #1 链头
			AuditRecord{
				ActorID: actorID, ActorType: PrincipalPlatformAdmin,
				SourceTenant: srcTenant, TargetTenant: uuid.Nil, // same-tenant
				ResourceType: "users", Action: "read",
				RiskLevel: RiskLow, Decision: "allow", Reason: "same-tenant operation",
			},
			"allow",
		},
		{ // #2 跨租户 allow
			AuditRecord{
				ActorID: actorID, ActorType: PrincipalPlatformAdmin,
				SourceTenant: srcTenant, TargetTenant: tgtTenant,
				ResourceType: "organizations", Action: "read",
				RiskLevel: RiskMedium, Decision: "allow", Reason: "scope validation passed",
			},
			"allow",
		},
		{ // #3 跨租户 deny（ticket 缺失）
			AuditRecord{
				ActorID: actorID, ActorType: PrincipalPlatformAdmin,
				SourceTenant: srcTenant, TargetTenant: tgtTenant,
				ResourceType: "users", Action: "impersonate",
				RiskLevel: RiskCritical, Decision: "deny", Reason: "critical-risk impersonate operation requires an approved ticket",
			},
			"deny",
		},
	} {
		reqCtx := infrapg.WithTenantScope(ctx, "", "", infrapg.TenantFilterSystem)
		writer.Append(reqCtx, tc.rec)
		assert.Equal(t, tc.expect, tc.rec.Decision, "record #%d decision", i+1)
	}

	// ── 读回验证（用 SET SESSION 保持 session-level setting）──
	readConn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer readConn.Release()
	// set_tenant_context 是 transaction-level，跨事务就丢了。
	// 这里用 set_config SET SESSION (is_local=false) 保持整个连接生命周期有效。
	if _, err := readConn.Exec(ctx,
		`SELECT set_config('app.current_role', 'super_admin', false),
		        set_config('app.current_organization_id', '', false)`); err != nil {
		t.Fatal(err)
	}

	rows, err := readConn.Query(ctx,
		`SELECT actor_id, source_tenant, target_tenant, prev_hash, hash,
		        actor_type, resource_type, action, risk_level, decision, reason, created_at
		 FROM guliangiam.authorization_audit_records ORDER BY created_at ASC`)
	require.NoError(t, err)
	defer rows.Close()

	type auditRow struct {
		actorID      uuid.UUID
		sourceTenant *uuid.UUID
		targetTenant *uuid.UUID
		prevHash     *string
		hash         string
		actorType    string
		resourceType string
		action       string
		riskLevel    string
		decision     string
		reason       string
		createdAt    time.Time
	}
	var records []auditRow
	for rows.Next() {
		var r auditRow
		require.NoError(t, rows.Scan(&r.actorID, &r.sourceTenant, &r.targetTenant,
			&r.prevHash, &r.hash, &r.actorType, &r.resourceType, &r.action,
			&r.riskLevel, &r.decision, &r.reason, &r.createdAt))
		records = append(records, r)
	}
	require.Len(t, records, 3, "expected 3 audit records")

	// 链头：prev_hash 必须 NULL
	assert.Nil(t, records[0].prevHash, "first record prev_hash must be NULL")

	// 链式衔接 + hash 正确
	for i := 1; i < len(records); i++ {
		r := records[i]
		require.NotNil(t, r.prevHash, "record %d prev_hash must not be nil", i)
		assert.Equal(t, records[i-1].hash, *r.prevHash,
			"record %d prev_hash must equal record %d hash", i, i-1)

		// 重新计算 hash 对比（用 DB 里存的 created_at + 完整字段）
		src, tgt := uuid.Nil, uuid.Nil
		if r.sourceTenant != nil {
			src = *r.sourceTenant
		}
		if r.targetTenant != nil {
			tgt = *r.targetTenant
		}
		rec := AuditRecord{
			ActorID: r.actorID, ActorType: PrincipalType(r.actorType),
			SourceTenant: src, TargetTenant: tgt,
			ResourceType: r.resourceType, Action: r.action,
			RiskLevel: RiskLevel(r.riskLevel), Decision: r.decision, Reason: r.reason,
		}
		computed := computeHash(r.prevHash, r.createdAt.UTC().Truncate(time.Microsecond), rec)
		assert.Equal(t, r.hash, computed,
			"record %d hash mismatch (DB stored vs recomputed)", i)
	}

	// chain_state.last_hash 必须等于最后一条 hash
	var lastHash *string
	err = readConn.QueryRow(ctx,
		`SELECT last_hash FROM guliangiam.authorization_chain_state WHERE id = 1`).Scan(&lastHash)
	require.NoError(t, err)
	require.NotNil(t, lastHash, "chain_state.last_hash must not be nil after 3 appends")
	assert.Equal(t, records[len(records)-1].hash, *lastHash, "chain_state.last_hash must equal last audit record hash")
}
