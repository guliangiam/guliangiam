package invitations

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
)

type ListInvitationsQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Status         string
}

func (s *Service) ListInvitations(ctx context.Context, query ListInvitationsQuery) ([]*InvitationDTO, int64, error) {
	items, total, err := s.UOW.Invitations().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]*InvitationDTO, len(items))
	for i, inv := range items {
		dtos[i] = toInvitationDTO(inv)
	}
	return dtos, total, nil
}
