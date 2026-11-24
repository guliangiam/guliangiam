package regions

import (
	"context"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// RegionResolver 是 ADR-031 Decision 1 的域边界接口。
// 给定一个 orgID，解析其所在 region_code。
//
// 实现位置：
//   - infra/mysql/regions + infra/postgres/regions: 从 tenant_instances JOIN deployment_regions 查
//   - 测试用 StubRegionResolver: 内存 map[orgID]region_code
//
// DR-001 验收核心：所有跨 orgID 的数据查询都必须先通过 CrossRegionGuard.Validate。
type RegionResolver interface {
	// Resolve 返回 org 所在 region_code。
	// orgID 不存在或无绑定 → apperrors.NewNotFound("tenant_instances for org %s", orgID)
	Resolve(ctx context.Context, orgID string) (string, error)

	// ResolveBatch 批量解析，用于交叉比对场景。
	// 返回 map[orgID]region_code；不存在的 orgID 不会出现在 map 里。
	ResolveBatch(ctx context.Context, orgIDs []string) (map[string]string, error)
}

// CrossRegionGuard 封装 DR-001 数据驻留强约束。
//
// 语义：所有跨 org 的数据读/写操作必须先调 Validate。
// 如果两个 org 在不同 region，直接返回 CrossRegionError，
// 调用方（HTTP handler / Worker / Dispatcher）决定如何处置（通常是 403 Forbidden + audit log）。
//
// CrossRegionGuard 不做真正的数据路由 — 它只做域边界校验。
// 路由由平台层（未来版本化）完成：给定 region_code → 选对应物理 DB。
type CrossRegionGuard struct {
	resolver RegionResolver
}

// NewCrossRegionGuard 构造一个 CrossRegionGuard。
func NewCrossRegionGuard(resolver RegionResolver) *CrossRegionGuard {
	return &CrossRegionGuard{resolver: resolver}
}

// Validate 检查 orgA 和 orgB 是否在同一 region。
// 返回 nil 表示同 region（允许操作），
// 返回 *CrossRegionError 表示跨 region（操作被 DR-001 拦截）。
//
// operation 用于日志可读性（例如 "role.binding.create" / "user.list"）。
//
// 边界情况：
//   - orgA == orgB → 直接返回 nil（同一 org 内部操作不受限制）
//   - 任一 org 无 region 绑定 → apperrors.NotFound
func (g *CrossRegionGuard) Validate(ctx context.Context, orgA, orgB, operation string) error {
	if orgA == "" || orgB == "" {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "orgA and orgB must be non-empty", map[string]any{"field": "organizationId", "expected": "non-empty"})
	}

	// 同一 org 内部 → 不受限制
	if orgA == orgB {
		return nil
	}

	codes, err := g.resolver.ResolveBatch(ctx, []string{orgA, orgB})
	if err != nil {
		return apperrors.WrapInternalf(err, "resolve regions for cross-region check")
	}

	codeA, ok := codes[orgA]
	if !ok {
		return apperrors.NewNotFoundf("region binding for org %s", orgA)
	}
	codeB, ok := codes[orgB]
	if !ok {
		return apperrors.NewNotFoundf("region binding for org %s", orgB)
	}

	if codeA != codeB {
		return &CrossRegionError{
			OrgA:        orgA,
			RegionCodeA: codeA,
			OrgB:        orgB,
			RegionCodeB: codeB,
			Operation:   operation,
		}
	}

	return nil
}

// ValidateMany 检查多个 orgID 是否都在同一 region。
// 适用于批量操作（例如 Outbox Worker fan-out 多 org）。
// 如果任一 org 与第一个 org region 不同 → CrossRegionError。
func (g *CrossRegionGuard) ValidateMany(ctx context.Context, orgIDs []string, operation string) error {
	if len(orgIDs) <= 1 {
		return nil
	}

	codes, err := g.resolver.ResolveBatch(ctx, orgIDs)
	if err != nil {
		return apperrors.WrapInternalf(err, "resolve regions for batch check")
	}

	baseOrg := orgIDs[0]
	baseCode, ok := codes[baseOrg]
	if !ok {
		return apperrors.NewNotFoundf("region binding for org %s", baseOrg)
	}

	for _, orgID := range orgIDs[1:] {
		code, ok := codes[orgID]
		if !ok {
			return apperrors.NewNotFoundf("region binding for org %s", orgID)
		}
		if code != baseCode {
			return &CrossRegionError{
				OrgA:        baseOrg,
				RegionCodeA: baseCode,
				OrgB:        orgID,
				RegionCodeB: code,
				Operation:   operation,
			}
		}
	}
	return nil
}
