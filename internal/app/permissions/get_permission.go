package permissions

import "context"

func (s *Service) GetPermission(ctx context.Context, organizationID, id string) (*PermissionDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// 租户谓词在仓储层显式生效；不能回落到系统级 GetByID。
	permission, err := s.UOW.Permissions().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	detail := toDetail(permission)
	return &detail, nil
}
