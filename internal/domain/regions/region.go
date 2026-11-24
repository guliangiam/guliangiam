// Package regions 实现 ADR-031 Decision 1 的多区域驻留域模型。
//
// 核心职责：
//  1. DeploymentRegion — 物理部署区域注册表条目
//  2. TenantInstance    — org → region 的绑定关系
//  3. RegionResolver    — 给定 orgID 解析其所在 region_code
//  4. CrossRegionGuard  — DR-001 负向约束：拦截跨区域数据查询
//
// 设计原则：
//   - 域模型纯 Go，无 DB 依赖
//   - RegionResolver 是端口接口（infra 层实现），Stub 用于测试
//   - CrossRegionGuard 只做域边界校验，不做真正的数据路由（路由由平台层完成）
package regions

import (
	"errors"
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DeploymentRegion 是 ADR-031 Decision 1 的物理部署区域注册表条目。
// 一个物理部署单元（K8s cluster / VM group）必须在此注册，
// 每个 org 通过 TenantInstance 绑定一个主区域。
type DeploymentRegion struct {
	ID          string
	RegionCode  string // 稳定短码: cn-north-1 / us-east-1 / eu-west-2
	Name        string // 展示名
	CountryCode string // ISO 3166-1 alpha-2
	Timezone    string
	IsPrimary   bool
	RTOMinutes  int // 灾难恢复目标
	RPOMinutes  int // 恢复点目标
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Validate 校验 DeploymentRegion 完整性。
func (r *DeploymentRegion) Validate() error {
	if r.RegionCode == "" {
		return errors.New("region_code is required")
	}
	if len(r.RegionCode) > 32 {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("region_code too long: %d > 32", len(r.RegionCode)),
			map[string]any{"field": "region_code", "value": len(r.RegionCode), "max": 32},
		)
	}
	if r.CountryCode == "" {
		return errors.New("country_code is required")
	}
	if len(r.CountryCode) != 2 {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("country_code must be ISO 3166-1 alpha-2 (2 chars), got %d", len(r.CountryCode)),
			map[string]any{"field": "country_code", "value": r.CountryCode, "format": "iso3166_1_alpha_2"},
		)
	}
	return nil
}

// TenantInstance 是 org → region 的绑定关系。
// 一个 org 绑定一个主实例 (is_primary=true)；未来可扩展多区域多活。
type TenantInstance struct {
	ID             string
	OrganizationID string
	RegionID       string
	RegionCode     string // 冗余查询字段（从 DeploymentRegion 回填）
	IsPrimary      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CrossRegionError 由 CrossRegionGuard 在检测到跨区域查询时返回。
// DR-001 验收: 该错误必须携带两个 region_code 让调用方记录日志。
type CrossRegionError struct {
	OrgA        string
	RegionCodeA string
	OrgB        string
	RegionCodeB string
	Operation   string // 操作类型（logical name）
}

func (e *CrossRegionError) Error() string {
	return fmt.Sprintf("region residency violation: cannot %s between org %s (region=%s) and org %s (region=%s)",
		e.Operation, e.OrgA, e.RegionCodeA, e.OrgB, e.RegionCodeB)
}

// IsCrossRegion 判断 error 是否为跨区域冲突。
func IsCrossRegion(err error) (*CrossRegionError, bool) {
	var ce *CrossRegionError
	if errors.As(err, &ce) {
		return ce, true
	}
	return nil, false
}

// _ ensures CrossRegionError 实现了 AppError 接口（如果有的话）。
var _ error = (*CrossRegionError)(nil)
