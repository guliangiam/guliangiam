package roles

import "context"

func (s *Service) GetRole(ctx context.Context, organizationID, id string) (*RoleDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	role, err := s.UOW.Roles().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	detail := toDetail(role)
	return &detail, nil
}
