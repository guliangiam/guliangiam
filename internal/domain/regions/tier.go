// Package regions 已存在 — 本文件新增 Tier 枚举 + 复制策略分类器 (ADR-031 Decision 2).
//
// DR-002 核心：数据按业务语义分 Tier，每个 Tier 有明确的跨 region 复制策略。
// 物理复制引擎（PostgreSQL streaming replication / Kafka mirror / ...）
// 不在本包 — 本包只负责"谁能/不能跨 region 复制"的域定义和写入拦截。
//
// 分类原则：
//   - 身份主数据 / 会话 / 令牌  → NeverCross  （DR-002 "区域自治"）
//   - 平台配置                   → QuasiRealtime （DR-002 "准实时复制"）
//   - 审计日志                   → AsyncAggregate （DR-002 "异步汇聚"）
//   - 计量事件                   → AsyncAggregate （DR-002 "异步汇聚"）
//
// DR-005 三档 RTO/RPO 与 Tier 独立：每个 deployment_regions 条目声明自己的 SLA，
// 本包不硬编码阈值（只声明 L1/L2/L3 语义枚举）。

package regions

import (
	"fmt"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Tier 是 ADR-031 Decision 2 定义的数据分类。
// 分类依据是业务语义，不是技术实现。
type Tier int

const (
	// TierIdentity: 身份主数据 + 会话/令牌
	// DR-002 策略：区域自治 — 每个 region 独立存储，不跨区域复制
	// 影响表：users / roles / groups / role_bindings / sessions / tokens / user_providers / link_accounts
	TierIdentity Tier = iota + 1

	// TierConfig: 平台配置
	// DR-002 策略：准实时复制 — 配置变更同步到所有 region
	// 影响表：organizations / providers / webhooks / connectors / security_policies / encryption_keys
	TierConfig

	// TierAudit: 审计日志
	// DR-002 策略：异步复制仅合规汇聚场景 — 本地实时写，异地只读汇聚归档
	// 影响表：records / audit_logs / audit_chain_anchors
	TierAudit

	// TierBilling: 计量/计费事件
	// DR-002 策略：异步汇聚控制面 — 所有 region 的计量事件汇总到 billing 控制面
	// 影响表：meters / bills / invoices / billing_events
	TierBilling

	// TierSystem: schema / migrations / encryption_keys_fingerprints 等系统元数据
	// DR-002 策略：NeverCross — 每个 region 独立 schema 演进
	TierSystem
)

// String 返回 Tier 的可读名。
func (t Tier) String() string {
	switch t {
	case TierIdentity:
		return "identity"
	case TierConfig:
		return "config"
	case TierAudit:
		return "audit"
	case TierBilling:
		return "billing"
	case TierSystem:
		return "system"
	default:
		return fmt.Sprintf("unknown(%d)", int(t))
	}
}

// ReplicationPolicy 是 ADR-031 Decision 2 定义的跨 region 复制策略。
type ReplicationPolicy int

const (
	// PolicyNeverCross: 区域自治 — 禁止跨 region 复制
	// 适用于 TierIdentity / TierSystem
	PolicyNeverCross ReplicationPolicy = iota + 1

	// PolicyQuasiRealtime: 准实时复制 — 配置变更同步到所有 region
	// 适用于 TierConfig
	PolicyQuasiRealtime

	// PolicyAsyncAggregate: 异步汇聚 — 本地实时写，异地只读汇聚归档
	// 适用于 TierAudit / TierBilling
	PolicyAsyncAggregate
)

// String 返回策略的可读名。
func (p ReplicationPolicy) String() string {
	switch p {
	case PolicyNeverCross:
		return "never-cross"
	case PolicyQuasiRealtime:
		return "quasi-realtime"
	case PolicyAsyncAggregate:
		return "async-aggregate"
	default:
		return fmt.Sprintf("unknown(%d)", int(p))
	}
}

// tierPolicyMap 是 Tier → ReplicationPolicy 的静态映射。
// ADR-031 Decision 2 语义，不可在运行期修改。
var tierPolicyMap = map[Tier]ReplicationPolicy{
	TierIdentity: PolicyNeverCross,
	TierConfig:   PolicyQuasiRealtime,
	TierAudit:    PolicyAsyncAggregate,
	TierBilling:  PolicyAsyncAggregate,
	TierSystem:   PolicyNeverCross,
}

// PolicyOf 返回 Tier 对应的 ReplicationPolicy。
func PolicyOf(t Tier) (ReplicationPolicy, error) {
	p, ok := tierPolicyMap[t]
	if !ok {
		return 0, apperrors.Newf(apperrors.CodeInternal, "unknown tier %s", t)
	}
	return p, nil
}

// ── 表名 → Tier 分类器 ──

// tableTierMap 是 table name → Tier 的静态映射。
// 表名不带 schema 前缀（只写 "users" 不写 "guliangiam.users"）。
// ADR-031 Decision 2 语义：新表必须在这里登记，否则 Classify 返回 ErrUnknownTable。
var tableTierMap = map[string]Tier{
	// ── TierIdentity: 区域自治（不跨 region）──
	"users":              TierIdentity,
	"roles":              TierIdentity,
	"groups":             TierIdentity,
	"user_role_bindings": TierIdentity,
	"group_members":      TierIdentity,
	"sessions":           TierIdentity,
	"tokens":             TierIdentity,
	"link_accounts":      TierIdentity,

	// ── TierConfig: 准实时复制 ──
	"organizations":         TierConfig,
	"providers":             TierConfig,
	"webhooks":              TierConfig,
	"connectors":            TierConfig,
	"security_policies":     TierConfig,
	"organization_settings": TierConfig,
	"encryption_keys":       TierConfig,
	"deployment_regions":    TierConfig,
	"tenant_instances":      TierConfig,

	// ── TierAudit: 异步汇聚 ──
	"records":             TierAudit,
	"audit_chain_anchors": TierAudit,

	// ── TierBilling: 异步汇聚 ──
	"billing_events": TierBilling,

	// ── TierSystem: 永不跨 region ──
	"schema_migrations": TierSystem,
}

// ErrUnknownTable 在表名未登记在 tableTierMap 时返回。
// 定义为 apperrors sentinel，CodeInternal 因为"表名未登记"是配置/代码缺陷，不是用户错误。
var ErrUnknownTable = apperrors.New(apperrors.CodeInternal, "table not registered in replication classifier")

// Classify 按表名返回 Tier 和 ReplicationPolicy。
// tableName 不带 schema 前缀（只写 "users"）。
func Classify(tableName string) (Tier, ReplicationPolicy, error) {
	t, ok := tableTierMap[tableName]
	if !ok {
		return 0, 0, apperrors.Wrapf(apperrors.CodeInternal, ErrUnknownTable, "table: %s", tableName)
	}
	p, _ := PolicyOf(t) // PolicyOf 对已知 Tier 永不报错
	return t, p, nil
}

// ── DR-005 三档 RTO/RPO ──

// RecoveryTier 是 ADR-031 DR-005 定义的三档 RTO/RPO 语义等级。
type RecoveryTier int

const (
	// RecoveryTierCritical (L1): 关键业务
	// 目标：RTO < 1min / RPO = 0
	RecoveryTierCritical RecoveryTier = iota + 1

	// RecoveryTierImportant (L2): 重要业务
	// 目标：RTO < 5min / RPO < 5min
	RecoveryTierImportant

	// RecoveryTierStandard (L3): 标准业务
	// 目标：RTO < 4h / RPO < 24h
	RecoveryTierStandard
)

// String 返回可读名。
func (r RecoveryTier) String() string {
	switch r {
	case RecoveryTierCritical:
		return "L1-critical"
	case RecoveryTierImportant:
		return "L2-important"
	case RecoveryTierStandard:
		return "L3-standard"
	default:
		return fmt.Sprintf("unknown(%d)", int(r))
	}
}

// TargetMinutes 返回 (RTO_minutes, RPO_minutes) 目标值。
// RPO = -1 表示 0（零数据丢失）。
func (r RecoveryTier) TargetMinutes() (rtoMinutes, rpoMinutes int) {
	switch r {
	case RecoveryTierCritical:
		return 1, -1 // RPO = 0
	case RecoveryTierImportant:
		return 5, 5
	case RecoveryTierStandard:
		return 240, 1440 // 4h, 24h
	default:
		return 0, 0
	}
}

// TierRecoveryDefaults 是 Tier → 默认 RecoveryTier 的静态映射。
// identity/session/token → L1（会话丢了就不可恢复）
// audit → L1（审计不能丢）
// config → L2（配置丢失影响但可恢复）
// billing → L3（计量可补录）
var TierRecoveryDefaults = map[Tier]RecoveryTier{
	TierIdentity: RecoveryTierCritical,
	TierAudit:    RecoveryTierCritical,
	TierConfig:   RecoveryTierImportant,
	TierBilling:  RecoveryTierStandard,
	TierSystem:   RecoveryTierCritical,
}
