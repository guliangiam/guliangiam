package gateway

import (
	"context"
)

func (s *Service) GetServer(ctx context.Context, organizationID, id string) (*ServerDTO, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// id comes from the URL path; the lookup is tenant-scoped so a guessed
	// foreign id is indistinguishable from a missing one.
	server, err := s.UOW.Gateway().GetServerByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	return toServerDTO(server), nil
}
