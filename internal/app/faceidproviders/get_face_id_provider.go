package faceidproviders

import "context"

func (s Service) GetFaceIdProvider(ctx context.Context, organizationID, id string) (*FaceIdProviderDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	provider, err := s.UOW.FaceIdProviders().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	detail := toDetail(provider)
	return &detail, nil
}
