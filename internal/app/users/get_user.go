package users

import "context"

func (s *Service) GetUser(ctx context.Context, id, organizationID string) (*UserDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// Management by-ID reads resolve ownership before touching any row: a
	// foreign user is indistinguishable from a missing one.
	user, err := s.UOW.Users().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	organization, err := s.UOW.Organizations().GetByID(ctx, user.OrganizationID)
	if err != nil {
		return nil, err
	}

	detail := toDetail(user, organization)

	// webauthn_credentials 无组织列，父用户即租户锚点（webauthn_admin.go 同
	// 口径）；scoped 读已在上方完成归属证明。
	credentials, err := s.UOW.WebAuthnCredentials().ListByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	detail.WebauthnCredentialCount = len(credentials)

	return &detail, nil
}
