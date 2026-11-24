package applications

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

type ListApplicationsQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Category       string
	Status         string
}

func (s *Service) ListApplications(ctx context.Context, query ListApplicationsQuery) ([]ApplicationListItem, int64, error) {
	// query.OrganizationID 经 chi.go 的 TrustedOrganizationID 收敛；List 按该 org 过滤（空则不加条件），可见行由调用方作用域与 RLS 决定。
	applications, total, err := s.UOW.Applications().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}

	items := make([]ApplicationListItem, 0, len(applications))
	for _, application := range applications {
		items = append(items, toListItem(application))
	}

	return items, total, nil
}

// ApplicationNameItem is a lightweight DTO for application name listing.
type ApplicationNameItem struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
}

// GetApplicationNames returns a lightweight list of all application names.
func (s *Service) GetApplicationNames(ctx context.Context) ([]ApplicationNameItem, error) {
	// 无 org 条件（同 ListApplications 的空 OrganizationID 语义），返回当前作用域内的应用名录。
	// /applications/names is the picker catalog: it must contain every
	// application in scope. A single PageSize:1000 call was clamped to 100 by
	// the repository, so the walk is explicit.
	applications, err := platformpagination.CollectAll(func(page, pageSize int) ([]*domain.Application, int64, error) {
		return s.UOW.Applications().List(ctx, domain.ListQuery{Page: page, PageSize: pageSize})
	})
	if err != nil {
		return nil, err
	}

	items := make([]ApplicationNameItem, 0, len(applications))
	for _, app := range applications {
		items = append(items, ApplicationNameItem{
			ID:          app.ID,
			Slug:        app.Slug,
			DisplayName: app.DisplayName,
		})
	}

	return items, nil
}
