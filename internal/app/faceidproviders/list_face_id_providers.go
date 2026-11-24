package faceidproviders

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
)

type ListFaceIdProvidersQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	IsEnabled      *bool
}

func (s Service) ListFaceIdProviders(ctx context.Context, query ListFaceIdProvidersQuery) ([]FaceIdProviderListItem, int64, error) {
	providers, total, err := s.UOW.FaceIdProviders().List(ctx, domain.ListQuery{
		Page:           query.Page,
		PageSize:       query.PageSize,
		OrganizationID: query.OrganizationID,
		IsEnabled:      query.IsEnabled,
	})
	if err != nil {
		return nil, 0, err
	}

	items := make([]FaceIdProviderListItem, 0, len(providers))
	for _, provider := range providers {
		items = append(items, toListItem(provider))
	}

	return items, total, nil
}
