package gateway

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
)

func (s *Service) ListServers(ctx context.Context, query ListServersQueryApp) ([]*ServerDTO, int64, error) {
	// organizationId 由 transport 层用 TrustedOrganizationID 收敛；仓储按
	// organization_id 过滤，空值在 handler 层已拒绝。
	items, total, err := s.UOW.Gateway().ListServers(ctx, domain.ListServersQuery(query))
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]*ServerDTO, len(items))
	for i, srv := range items {
		dtos[i] = toServerDTO(srv)
	}
	return dtos, total, nil
}
