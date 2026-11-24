package applications

import (
	"context"

	appdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// GetDefaultApplication returns the first visible application for the given organization
// as a full ApplicationDetail. Used by the frontend getUserApplication() API call
// which expects the complete application configuration (not just list-level fields).
func (s *Service) GetDefaultApplication(ctx context.Context, organizationID string) (*ApplicationDetail, error) {
	if organizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}

	// organizationID 由调用方传入（organizations/{id}/default-application 取 URL id，user-application 取 JWT claim）；List 按该 org 过滤，可见行由调用方作用域与 RLS 决定。
	apps, _, err := s.UOW.Applications().List(ctx, appdomain.ListQuery{
		OrganizationID: organizationID,
		Page:           1,
		PageSize:       1,
	})
	if err != nil {
		return nil, err
	}
	if len(apps) == 0 {
		return nil, apperrors.NewNotFound("no default application found for organization")
	}

	detail := toDetail(apps[0])
	keys, err := s.UOW.Applications().ListActiveClientJWKs(ctx, apps[0].OrganizationID, apps[0].ID)
	if err != nil {
		return nil, err
	}
	detail.ClientJWKS = projectClientJWKS(keys)
	uri, err := s.clientJWKSURI(ctx, apps[0].OrganizationID, apps[0].ID)
	if err != nil {
		return nil, err
	}
	detail.JWKSURI = uri
	return detail, nil
}
