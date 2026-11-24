package groups

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
)

type ListGroupsQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Status         string
	Type           string
	SortField      string
	SortOrder      string
}

func (s *Service) ListGroups(ctx context.Context, query ListGroupsQuery) ([]GroupListItem, int64, error) {
	groups, total, err := s.UOW.Groups().List(ctx, domain.ListQuery{
		Page:           query.Page,
		PageSize:       query.PageSize,
		OrganizationID: query.OrganizationID,
		Keyword:        query.Keyword,
		Status:         query.Status,
		Type:           query.Type,
		SortField:      query.SortField,
		SortOrder:      query.SortOrder,
	})
	if err != nil {
		return nil, 0, err
	}

	items := make([]GroupListItem, 0, len(groups))
	for _, group := range groups {
		items = append(items, toListItem(group))
	}

	return items, total, nil
}
