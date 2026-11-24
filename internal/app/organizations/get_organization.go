package organizations

import "context"

func (s *Service) GetOrganization(ctx context.Context, id string) (*OrganizationDetail, error) {
	// organizations 表 qual 拦截 SELECT（id != current_organization_id()）。
	organization, err := s.UOW.Organizations().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	detail := toDetail(organization)
	return &detail, nil
}

func (s *Service) GetOrganizationBySlug(ctx context.Context, slug string) (*OrganizationDetail, error) {
	organization, err := s.UOW.Organizations().GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	detail := toDetail(organization)
	return &detail, nil
}

// GetOrganizationBySlugPublic serves the anonymous discovery endpoints
// (/by-slug, /resolve). It projects the aggregate onto OrganizationPublicDetail
// so credential-bearing fields never reach an unauthenticated caller.
func (s *Service) GetOrganizationBySlugPublic(ctx context.Context, slug string) (*OrganizationPublicDetail, error) {
	organization, err := s.UOW.Organizations().GetBySlugPublic(ctx, slug)
	if err != nil {
		return nil, err
	}

	detail := toPublicDetail(organization)
	return &detail, nil
}
