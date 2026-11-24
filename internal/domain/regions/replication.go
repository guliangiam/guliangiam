package regions

import (
	"context"
	"errors"
	"fmt"
)

// ReplicationViolationError 由 RegionResidentPolicy 在检测到不该跨 region 的写入时返回。
// 与 Decision 1 的 CrossRegionError 不同：
//   - CrossRegionError: 拦截**跨 org 操作**（domain 边界）
//   - ReplicationViolationError: 拦截**不该跨 region 的表数据写入**（DR-002 复制策略）
type ReplicationViolationError struct {
	Table     string
	Tier      Tier
	Policy    ReplicationPolicy
	SourceOrg string
	TargetOrg string
	Reason    string
}

func (e *ReplicationViolationError) Error() string {
	return fmt.Sprintf("replication violation: table %s (tier=%s, policy=%s) cannot replicate from org %s to org %s: %s",
		e.Table, e.Tier, e.Policy, e.SourceOrg, e.TargetOrg, e.Reason)
}

// IsReplicationViolation 判断 error 是否为复制策略冲突。
func IsReplicationViolation(err error) (*ReplicationViolationError, bool) {
	var ve *ReplicationViolationError
	if errors.As(err, &ve) {
		return ve, true
	}
	return nil, false
}

// _ 满足 error 接口
var _ error = (*ReplicationViolationError)(nil)

// RegionResidentPolicy 是 Decision 2 + Decision 1 的组合拦截器。
//
// 使用场景：Outbox Worker 跨 region fan-out / 数据迁移脚本 / ETL 跨 region 同步。
// 任何"数据从 orgA 写入到 orgB"的操作都必须先调 CanReplicate。
//
// 语义：
//  1. orgA == orgB → 直接通过（同一 org 内部写入不受限）
//  2. Classify(table) → Tier → Policy
//  3. PolicyNeverCross → **必须**同 region（调 CrossRegionGuard.Validate）
//  4. PolicyQuasiRealtime / AsyncAggregate → 允许跨 region（由复制引擎负责物理投递）
type RegionResidentPolicy struct {
	crossGuard *CrossRegionGuard
}

// NewRegionResidentPolicy 构造一个 RegionResidentPolicy。
// 注入 CrossRegionGuard（Decision 1 的实现）。
func NewRegionResidentPolicy(crossGuard *CrossRegionGuard) *RegionResidentPolicy {
	return &RegionResidentPolicy{crossGuard: crossGuard}
}

// CanReplicate 检查 table 的数据能否从 sourceOrg 复制到 targetOrg。
//
// 返回 nil → 允许；返回 ReplicationViolationError → 拦截。
//
// boundary 用于日志可读性（例如 "outbox.fanout" / "etl.migration"）。
func (p *RegionResidentPolicy) CanReplicate(ctx context.Context, table, sourceOrg, targetOrg, boundary string) error {
	// 同一 org 内部 → 直接通过
	if sourceOrg == targetOrg {
		return nil
	}

	tier, policy, err := Classify(table)
	if err != nil {
		// 表未登记 → 默认 NeverCross（安全优先）
		return &ReplicationViolationError{
			Table:     table,
			Tier:      TierIdentity,
			Policy:    PolicyNeverCross,
			SourceOrg: sourceOrg,
			TargetOrg: targetOrg,
			Reason:    fmt.Sprintf("table not registered; defaulting to never-cross policy: %v", err),
		}
	}

	// PolicyNeverCross → 必须同 region
	if policy == PolicyNeverCross {
		if err := p.crossGuard.Validate(ctx, sourceOrg, targetOrg, boundary); err != nil {
			return &ReplicationViolationError{
				Table:     table,
				Tier:      tier,
				Policy:    policy,
				SourceOrg: sourceOrg,
				TargetOrg: targetOrg,
				Reason:    fmt.Sprintf("policy=never-cross requires same region, cross-region check failed: %v", err),
			}
		}
		return nil // 同 region → NeverCross 表可以复制
	}

	// PolicyQuasiRealtime / AsyncAggregate → 物理复制引擎负责，域层不拦截
	return nil
}
