package roles

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
)

type ListRolesQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

func (s *Service) ListRoles(ctx context.Context, query ListRolesQuery) ([]RoleListItem, int64, error) {
	roles, total, err := s.UOW.Roles().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}

	items := make([]RoleListItem, 0, len(roles))
	for _, role := range roles {
		items = append(items, toListItem(role))
	}

	return items, total, nil
}
