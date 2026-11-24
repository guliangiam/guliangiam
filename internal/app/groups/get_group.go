package groups

import "context"

func (s *Service) GetGroup(ctx context.Context, organizationID, id string) (*GroupDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	group, err := s.UOW.Groups().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	return toDetail(group), nil
}
