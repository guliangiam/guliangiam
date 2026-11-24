package permissions

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
)

type ListPermissionsQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
	State          string
	ResourceType   string
	Effect         string
}

func (s *Service) ListPermissions(ctx context.Context, query ListPermissionsQuery) ([]PermissionListItem, int64, error) {
	permissions, total, err := s.UOW.Permissions().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}

	items := make([]PermissionListItem, 0, len(permissions))
	for _, permission := range permissions {
		items = append(items, toListItem(permission))
	}

	return items, total, nil
}
