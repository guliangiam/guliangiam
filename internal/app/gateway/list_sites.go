package gateway

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
)

func (s *Service) ListSites(ctx context.Context, query ListSitesQueryApp) ([]*SiteDTO, int64, error) {
	// organizationId 直接取自 query（未做 TrustedOrganizationID 收敛）；传空时 SQL 不加 org 条件，返回作用域内可见行。
	items, total, err := s.UOW.Gateway().ListSites(ctx, domain.ListSitesQuery(query))
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]*SiteDTO, len(items))
	for i, site := range items {
		dtos[i] = toSiteDTO(site)
	}
	return dtos, total, nil
}
