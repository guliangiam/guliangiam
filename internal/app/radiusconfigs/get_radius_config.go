package radiusconfigs

import "context"

func (s Service) GetRadiusConfig(ctx context.Context, organizationID, id string) (*RadiusConfigDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	config, err := s.UOW.RadiusConfigs().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	detail := toDetail(config)
	return &detail, nil
}
