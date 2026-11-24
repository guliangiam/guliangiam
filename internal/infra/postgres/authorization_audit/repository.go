package auditrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "github.com/guliangiam/guliangiam/internal/domain/authorization_audit"
	pgtenant "github.com/guliangiam/guliangiam/internal/infra/postgres"
)

// Repository — AuditRepository 的 Postgres 实现。
// 持有 *pgxpool.Pool（Append 内部自管事务）。
// 所有方法自动注入 TenantFilterSystem ctx（RLS: is_super_admin() 才能读写）。
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return Repository{pool: pool}
}

// sysCtx 注入 TenantFilterSystem — RLS: is_super_admin() 才能读写审计表。
func sysCtx(ctx context.Context) context.Context {
	return pgtenant.WithTenantScope(ctx, "", "", pgtenant.TenantFilterSystem)
}

// 确保实现 domain 接口。
var _ domain.AuditRepository = Repository{}

// Append 完成完整哈希链写入事务。
//
// 事务步骤：
//  1. BEGIN（TenantFilterSystem ctx）
//  2. SELECT last_hash FOR UPDATE — 锁单行 id=1
//  3. hash = sha256(prev_hash|created_at|actor_id|actor_type|...|reason) — 固定字段顺序
//  4. INSERT INTO audit_records(..., prev_hash, hash, created_at)
//  5. UPDATE chain_state SET last_hash = hash WHERE id=1
//  6. COMMIT
func (r Repository) Append(ctx context.Context, rec *domain.AuditRecord, createdAt string) error {
	sysCtx := sysCtx(ctx)

	// BEGIN
	tx, err := r.pool.Begin(sysCtx)
	if err != nil {
		return err
	}
	defer func() {
		if rbErr := tx.Rollback(sysCtx); rbErr != nil && !strings.Contains(rbErr.Error(), "tx is closed") {
			// Rollback 失败不阻塞 — 事务已被 COMMIT 或 ROLLBACK 标记为 closed；
			// 其余错误仅用于排障，不影响主流程返回值。
			slog.DebugContext(sysCtx, "authorization audit rollback failed", "error", rbErr)
		}
	}()

	// set_tenant_context：RLS 要求 app.current_role=super_admin 才能 INSERT 审计表
	if _, err := tx.Exec(sysCtx, `SELECT guliangiam.set_tenant_context(NULL::uuid, 'super_admin')`); err != nil {
		return err
	}

	// Step 1: 初始化并锁定链头（全新库首条写入也能进入同一串行临界区）。
	if _, err := tx.Exec(sysCtx, queryChainStateInit); err != nil {
		return err
	}
	var prevHash *string
	if err := tx.QueryRow(sysCtx, queryChainStateLock).Scan(&prevHash); err != nil {
		return err
	}

	// Step 2: 计算 SHA-256 hash（固定字段顺序 + 管道符分隔）
	hash := computeChainHash(prevHash, createdAt, rec)

	// Step 3: INSERT audit record
	var actorID, sourceTenant, targetTenant any
	actorID = rec.ActorID
	if rec.SourceTenant != nil && *rec.SourceTenant != uuid.Nil {
		sourceTenant = rec.SourceTenant
	}
	if rec.TargetTenant != nil && *rec.TargetTenant != uuid.Nil {
		targetTenant = rec.TargetTenant
	}

	if _, err := tx.Exec(sysCtx, queryInsertAuditRecord,
		actorID, rec.ActorType, sourceTenant, targetTenant,
		rec.ResourceType, rec.Action, rec.RiskLevel,
		rec.Decision, rec.Reason, prevHash, hash, createdAt,
	); err != nil {
		return err
	}

	// Step 4: UPDATE chain_state
	if _, err := tx.Exec(sysCtx, queryUpdateChainState, hash); err != nil {
		return err
	}

	// Step 5: COMMIT
	return tx.Commit(sysCtx)
}

// ListRecords 拉审计链记录（用于 validate chain 测试/验证）。
func (r Repository) ListRecords(ctx context.Context, limit int) ([]domain.ChainRecord, error) {
	sysCtx := sysCtx(ctx)
	rows, err := r.pool.Query(sysCtx, queryListForValidate, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []domain.ChainRecord
	for rows.Next() {
		var cr domain.ChainRecord
		if err := rows.Scan(
			&cr.ActorID, &cr.ActorType, &cr.SourceTenant, &cr.TargetTenant,
			&cr.ResourceType, &cr.Action, &cr.RiskLevel, &cr.Decision, &cr.Reason,
			&cr.PrevHash, &cr.Hash, &cr.CreatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, cr)
	}
	return records, rows.Err()
}

// computeChainHash 计算审计记录的哈希。
//
// 输入顺序固定（UTF-8 + 管道符分隔）：
//
//	prev_hash|created_at_iso8601|actor_id|actor_type|source_tenant|target_tenant|
//	resource_type|action|risk_level|decision|reason
//
// 空值用空字符串，uuid.Nil → 空字符串。
func computeChainHash(prevHash *string, createdAt string, rec *domain.AuditRecord) string {
	parts := []string{
		derefOrEmpty(prevHash),
		createdAt,
		uuidOrEmpty(rec.ActorID),
		rec.ActorType,
		uuidPtrOrEmpty(rec.SourceTenant),
		uuidPtrOrEmpty(rec.TargetTenant),
		rec.ResourceType,
		rec.Action,
		rec.RiskLevel,
		rec.Decision,
		derefOrEmpty(rec.Reason),
	}

	joined := strings.Join(parts, "|")
	h := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(h[:])
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func uuidOrEmpty(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

func uuidPtrOrEmpty(id *uuid.UUID) string {
	if id == nil || *id == uuid.Nil {
		return ""
	}
	return id.String()
}
