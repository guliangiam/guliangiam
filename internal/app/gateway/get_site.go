package gateway

import (
	"context"
)

func (s *Service) GetSite(ctx context.Context, organizationID, id string) (*SiteDTO, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// id comes from the URL path; the lookup is tenant-scoped so a guessed
	// foreign id is indistinguishable from a missing one.
	site, err := s.UOW.Gateway().GetSiteByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	return toSiteDTO(site), nil
}
