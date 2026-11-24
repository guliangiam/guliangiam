package invitations

import (
	"context"
)

func (s *Service) GetInvitation(ctx context.Context, organizationID, id string) (*InvitationDTO, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// The management by-ID route is tenant-bound; code-based accept/verify
	// flows keep using the system-scoped GetByID.
	inv, err := s.UOW.Invitations().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	return toInvitationDTO(inv), nil
}
