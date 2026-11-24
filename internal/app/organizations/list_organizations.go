package organizations

import (
	"context"
	"errors"
	"strings"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
)

type ListOrganizationsQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Status   string
}

func (s *Service) ListOrganizations(ctx context.Context, query ListOrganizationsQuery) ([]OrganizationListItem, int64, error) {
	// targeted/unknown 平台管理员在 RLS 提升后只能看见本次显式授权的租户；
	// 没有显式 target 时返回空集（不是全量目录）。
	if organizations, scoped, err := s.listOrganizationsInAdminScope(ctx); err != nil {
		return nil, 0, err
	} else if scoped {
		if len(organizations) == 0 || !matchesOrganizationListQuery(organizations[0], query) {
			return []OrganizationListItem{}, 0, nil
		}
		return []OrganizationListItem{toListItem(organizations[0])}, 1, nil
	}

	// 管理后台与 OrganizationSelect 的数据源：SQL 无 org 条件，普通会话只返回本 org（RLS），
	// all 作用域 platform_admin 会话返回全部。
	organizations, total, err := s.UOW.Organizations().List(ctx, orgdomain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}

	items := make([]OrganizationListItem, 0, len(organizations))
	for _, organization := range organizations {
		items = append(items, toListItem(organization))
	}

	return items, total, nil
}

// listOrganizationsInAdminScope 返回本次请求允许平台管理员看到的组织集合。
//
//	scoped=false → 不是受限作用域（普通用户 / all / 未分类），调用方沿用原查询。
//	scoped=true  → 作用域为 targeted/none/unknown：有显式授权 target 时只返回该组织，
//	               没有显式 target 时返回空集。
func (s *Service) listOrganizationsInAdminScope(ctx context.Context) ([]*orgdomain.Organization, bool, error) {
	scope, ok := scopeauthorizer.AdminScopeFromContext(ctx)
	if !ok || scope.GrantsAllTenants() {
		return nil, false, nil
	}

	target, hasTarget := scopeauthorizer.AuthorizedTargetTenantFromContext(ctx)
	if !hasTarget {
		return nil, true, nil
	}

	organization, err := s.UOW.Organizations().GetByID(ctx, target.String())
	if err != nil {
		var appErr *apperrors.Error
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return nil, true, nil
		}
		return nil, true, err
	}
	if organization == nil {
		return nil, true, nil
	}
	return []*orgdomain.Organization{organization}, true, nil
}

// matchesOrganizationListQuery 复刻仓储 List 的过滤语义（slug/display_name 模糊、
// status 精确），用于受限作用域下只做单行读取时保持同一查询结果。
func matchesOrganizationListQuery(organization *orgdomain.Organization, query ListOrganizationsQuery) bool {
	if query.Status != "" && organization.Status != query.Status {
		return false
	}
	if query.Keyword == "" {
		return true
	}
	keyword := strings.ToLower(query.Keyword)
	return strings.Contains(strings.ToLower(organization.Slug), keyword) ||
		strings.Contains(strings.ToLower(organization.DisplayName), keyword)
}
