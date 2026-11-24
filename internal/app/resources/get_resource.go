package resources

import "context"

func (s Service) GetResource(ctx context.Context, organizationID, id string) (*ResourceDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	r, err := s.UOW.Resources().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	detail := toDetail(r)
	return &detail, nil
}
