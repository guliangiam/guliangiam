package resources

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/resources"
)

type ListResourcesQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	ApplicationID  string
	Keyword        string
	Type           string
	Tag            string
}

func (s Service) ListResources(ctx context.Context, query ListResourcesQuery) ([]ResourceListItem, int64, error) {
	resources, total, err := s.UOW.Resources().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}

	items := make([]ResourceListItem, 0, len(resources))
	for _, r := range resources {
		items = append(items, toListItem(r))
	}

	return items, total, nil
}
