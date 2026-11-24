package radiusconfigs

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
)

type ListRadiusConfigsQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	IsEnabled      *bool
	AuthProtocol   string
}

func (s Service) ListRadiusConfigs(ctx context.Context, query ListRadiusConfigsQuery) ([]RadiusConfigListItem, int64, error) {
	configs, total, err := s.UOW.RadiusConfigs().List(ctx, domain.ListQuery{
		Page:           query.Page,
		PageSize:       query.PageSize,
		OrganizationID: query.OrganizationID,
		IsEnabled:      query.IsEnabled,
		AuthProtocol:   query.AuthProtocol,
	})
	if err != nil {
		return nil, 0, err
	}

	items := make([]RadiusConfigListItem, 0, len(configs))
	for _, config := range configs {
		items = append(items, toListItem(config))
	}

	return items, total, nil
}
