package organizations

import (
	"context"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// OrganizationNameItem is a lightweight DTO for organization name listings.
type OrganizationNameItem struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Slug        string `json:"slug"`
}

// GetOrganizationNames returns all organization names (lightweight, no pagination).
func (s *Service) GetOrganizationNames(ctx context.Context) ([]OrganizationNameItem, error) {
	// targeted/unknown 平台管理员的名录同样收敛：有显式授权 target 时只返回该组织，
	// 否则返回空集，避免 RLS 提升后经 /names 枚举全量目录。
	if organizations, scoped, err := s.listOrganizationsInAdminScope(ctx); err != nil {
		return nil, err
	} else if scoped {
		items := make([]OrganizationNameItem, 0, len(organizations))
		for _, org := range organizations {
			items = append(items, OrganizationNameItem{
				ID:          org.ID,
				DisplayName: org.DisplayName,
				Slug:        org.Slug,
			})
		}
		return items, nil
	}

	// SQL 无 org 条件；普通会话只返回本 org，all 作用域 platform_admin 会话返回全部组织名录。
	// 名录是选择器数据源，必须覆盖作用域内全部组织：单次 PageSize:1000 会被
	// 仓储按 MaxPageSize 静默截断到 100，因此按服务端上限逐页收集。
	orgs, err := platformpagination.CollectAll(func(page, pageSize int) ([]*orgdomain.Organization, int64, error) {
		return s.UOW.Organizations().List(ctx, orgdomain.ListQuery{Page: page, PageSize: pageSize})
	})
	if err != nil {
		return nil, err
	}

	items := make([]OrganizationNameItem, 0, len(orgs))
	for _, org := range orgs {
		items = append(items, OrganizationNameItem{
			ID:          org.ID,
			DisplayName: org.DisplayName,
			Slug:        org.Slug,
		})
	}
	return items, nil
}
