package scopeauthorizer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	auditrepo "github.com/guliangiam/guliangiam/internal/domain/authorization_audit"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AuditRecord 审计记录参数。
// 与 authorization_audit_records 表对齐。
type AuditRecord struct {
	ActorID      uuid.UUID
	ActorType    PrincipalType
	SourceTenant uuid.UUID
	TargetTenant uuid.UUID

	ResourceType string
	Action       string
	RiskLevel    RiskLevel

	Decision string // "allow" | "deny"
	Reason   string
}

// AuditWriter 审计哈希链写入器。
//
// Phase C-Step 4 + Phase D hash-chain design:
//   - 所有 DB 操作下沉到 AuditRepository（内部自管事务 BEGIN → SELECT FOR UPDATE → 计算 hash → INSERT → UPDATE → COMMIT）
//   - created_at 应用侧显式 time.Now().UTC().Truncate(time.Microsecond)（experienceRecall 教训：避免 DB DEFAULT 不一致）
//   - SHA-256 固定字段顺序 + 管道符分隔 + UTF-8 编码（计算在 repository 内）
//   - 审计写入失败 → slog.Error + 不 return error（不阻塞授权主路径）
type AuditWriter struct {
	repo auditrepo.AuditRepository
	log  *slog.Logger
}

// NewAuditWriter 构造审计写入器。
func NewAuditWriter(repo auditrepo.AuditRepository, log *slog.Logger) *AuditWriter {
	return &AuditWriter{repo: repo, log: log}
}

// Append 写入一条审计记录。
//
// AuditRepository 内部自管完整哈希链事务（BEGIN → SELECT FOR UPDATE → 计算 hash → INSERT → UPDATE → COMMIT）。
// 应用层只负责：构造 AuditRecord 实体 + 生成 createdAt。
//
// 失败 → slog.Error 但不阻塞 ScopeAuthorizer.Authorize 的决策返回值。
func (w *AuditWriter) Append(ctx context.Context, rec AuditRecord) {
	if w.repo == nil {
		if w.log != nil {
			w.log.ErrorContext(ctx, "audit: no repository configured, skipping write")
		}
		return
	}

	// 应用侧生成 created_at（experienceRecall 教训：避免 DB DEFAULT 不一致）
	createdAt := time.Now().UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)

	// 构造 domain 实体
	sourceTenant := rec.SourceTenant
	targetTenant := rec.TargetTenant
	auditRec := &auditrepo.AuditRecord{
		ActorID:      rec.ActorID,
		ActorType:    string(rec.ActorType),
		SourceTenant: &sourceTenant,
		TargetTenant: &targetTenant,
		ResourceType: rec.ResourceType,
		Action:       rec.Action,
		RiskLevel:    string(rec.RiskLevel),
		Decision:     rec.Decision,
	}
	if rec.Reason != "" {
		reason := rec.Reason
		auditRec.Reason = &reason
	}

	// 交给 repository 完成完整哈希链事务
	if err := w.repo.Append(ctx, auditRec, createdAt); err != nil {
		w.logError(ctx, "audit: write failed", err)
		return
	}

	if w.log != nil {
		w.log.InfoContext(ctx, "audit: record written",
			"decision", rec.Decision,
			"actor_type", rec.ActorType,
			"resource_type", rec.ResourceType,
			"action", rec.Action,
			"risk_level", rec.RiskLevel,
		)
	}
}

// computeHash 计算审计记录的哈希（测试辅助 + 保持向后兼容）。
//
// 输入顺序固定（UTF-8 + 管道符分隔）：
//
//	prev_hash|created_at_iso8601|actor_id|actor_type|source_tenant|target_tenant|
//	resource_type|action|risk_level|decision|reason
//
// 空值用空字符串，uuid.Nil → 空字符串。
func computeHash(prevHash *string, createdAt time.Time, rec AuditRecord) string {
	parts := []string{
		derefOrEmpty(prevHash),
		createdAt.Format(time.RFC3339Nano),
		uuidOrEmpty(rec.ActorID),
		string(rec.ActorType),
		uuidOrEmpty(rec.SourceTenant),
		uuidOrEmpty(rec.TargetTenant),
		rec.ResourceType,
		rec.Action,
		string(rec.RiskLevel),
		rec.Decision,
		rec.Reason,
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

func (w *AuditWriter) logError(ctx context.Context, msg string, err error) {
	if w.log != nil {
		w.log.ErrorContext(ctx, msg, "error", err)
	}
}

// MarshalJSON 序列化 AuditRecord（用于结构化日志）。
func (r AuditRecord) MarshalJSON() ([]byte, error) {
	type alias AuditRecord
	return json.Marshal(struct {
		Decision string `json:"decision"`
		Action   string `json:"action"`
		Resource string `json:"resource_type"`
		Risk     string `json:"risk_level"`
		*alias
	}{
		Decision: r.Decision,
		Action:   r.Action,
		Resource: r.ResourceType,
		Risk:     string(r.RiskLevel),
		alias:    (*alias)(&r),
	})
}

// hashInputForTest 暴露给测试的 hash 输入拼接（方便断言）。
func hashInputForTest(prevHash *string, createdAt time.Time, rec AuditRecord) string {
	parts := []string{
		derefOrEmpty(prevHash),
		createdAt.Format(time.RFC3339Nano),
		uuidOrEmpty(rec.ActorID),
		string(rec.ActorType),
		uuidOrEmpty(rec.SourceTenant),
		uuidOrEmpty(rec.TargetTenant),
		rec.ResourceType,
		rec.Action,
		string(rec.RiskLevel),
		rec.Decision,
		rec.Reason,
	}
	return strings.Join(parts, "|")
}

// validateChain 验证哈希链完整性（给定 DB 中的记录序列）。
// 返回 nil 表示链有效，否则返回第一条不一致的错误信息。
func validateChain(records []chainRecord) error {
	var expectedPrevHash *string
	var prev *chainRecord
	for i := range records {
		rec := &records[i]
		if prev == nil {
			if rec.PrevHash != nil {
				return apperrors.NewInternalf("record[0]: expected prev_hash=nil, got %s", *rec.PrevHash)
			}
		} else {
			if expectedPrevHash == nil {
				return apperrors.NewInternalf("record[%d]: expected prev_hash=%s, got nil", i, prev.Hash)
			}
			if rec.PrevHash == nil || *rec.PrevHash != *expectedPrevHash {
				got := "<nil>"
				if rec.PrevHash != nil {
					got = *rec.PrevHash
				}
				return apperrors.NewInternalf("record[%d]: expected prev_hash=%s, got %s", i, *expectedPrevHash, got)
			}
		}

		recalc := recomputeHash(expectedPrevHash, *rec)
		if rec.Hash != recalc {
			return apperrors.NewInternalf("record[%d]: hash mismatch: stored=%s, computed=%s", i, rec.Hash, recalc)
		}

		next := rec.Hash
		expectedPrevHash = &next
		prev = rec
	}
	return nil
}

type chainRecord struct {
	PrevHash     *string
	Hash         string
	CreatedAt    time.Time
	ActorID      uuid.UUID
	ActorType    string
	SourceTenant uuid.UUID
	TargetTenant uuid.UUID
	ResourceType string
	Action       string
	RiskLevel    string
	Decision     string
	Reason       string
}

func recomputeHash(prevHash *string, rec chainRecord) string {
	parts := []string{
		derefOrEmpty(prevHash),
		rec.CreatedAt.Format(time.RFC3339Nano),
		uuidOrEmpty(rec.ActorID),
		rec.ActorType,
		uuidOrEmpty(rec.SourceTenant),
		uuidOrEmpty(rec.TargetTenant),
		rec.ResourceType,
		rec.Action,
		rec.RiskLevel,
		rec.Decision,
		rec.Reason,
	}
	joined := strings.Join(parts, "|")
	h := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(h[:])
}
