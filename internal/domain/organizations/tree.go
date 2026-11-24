package organizations

import (
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// MaxOrganizationTreeDepth 是组织树深度的默认上限（根=1）。设计声明「扁平化上限
// 与最大深度由策略声明」，当前实现先以默认值兜底，策略化覆盖在实现批次接入；
// 超限拒绝而非静默截断（design/b2b §组织树）。
const MaxOrganizationTreeDepth = 8

// ValidateOrganizationParent 校验一次 parent 变更是否会让组织树成环或超深。
// ancestors 是目标 parent 到根的祖先链（最近者在前），由调用方按租户作用域读取；
// parentID 为空表示把组织提升为根。返回的 *apperrors.Error 为 422。
func ValidateOrganizationParent(selfID, parentID string, ancestors []string) error {
	if parentID == "" {
		return nil
	}
	if parentID == selfID {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"organization cannot be its own parent",
			map[string]any{"field": "parentId", "value": parentID})
	}
	for _, ancestor := range ancestors {
		if ancestor == selfID {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"organization tree would contain a cycle",
				map[string]any{"field": "parentId", "value": parentID, "cycleAt": selfID})
		}
	}
	if depth := len(ancestors) + 1; depth > MaxOrganizationTreeDepth {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"organization tree depth exceeds the maximum",
			map[string]any{"field": "parentId", "depth": depth, "maxDepth": MaxOrganizationTreeDepth})
	}
	return nil
}

// TreeDepth 返回以 ancestors 为祖先链的组织所处深度（根=1）。
func TreeDepth(ancestors []string) int {
	return len(ancestors) + 1
}
