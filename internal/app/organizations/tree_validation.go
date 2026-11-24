package organizations

import (
	"context"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// validateOrganizationParentChain 读取 parent 到根的祖先链并交给域规则校验。
// parentID 为空表示提升为根；读取失败统一按 422 返回（不区分跨租户/不存在，
// 避免枚举），成环与深度由 orgdomain.ValidateOrganizationParent 判定。
func validateOrganizationParentChain(ctx context.Context, repo orgdomain.Repository, selfID, parentID string) error {
	if parentID == "" {
		return nil
	}
	ancestors := make([]string, 0, orgdomain.MaxOrganizationTreeDepth)
	current := parentID
	for len(ancestors) < orgdomain.MaxOrganizationTreeDepth {
		parent, err := repo.GetByID(ctx, current)
		if err != nil || parent == nil {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"parent organization not found",
				map[string]any{"field": "parentId", "value": current})
		}
		ancestors = append(ancestors, parent.ID)
		if parent.ParentID == "" {
			break
		}
		current = parent.ParentID
	}
	return orgdomain.ValidateOrganizationParent(selfID, parentID, ancestors)
}
