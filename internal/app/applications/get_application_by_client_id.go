package applications

import (
	"context"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// GetApplicationByClientID looks up an application by its OAuth client_id and
// returns the full management detail. The anonymous by-client-id route uses
// GetApplicationPublicByClientID instead; management by-ID routes must use
// GetByIDForOrganization.
func (s *Service) GetApplicationByClientID(ctx context.Context, clientID string) (*ApplicationDetail, error) {
	// GetByClientID has no organization_id predicate; visibility is decided by
	// the caller scope and RLS, same as the public discovery route below.
	application, err := s.UOW.Applications().GetByClientID(ctx, clientID)
	if err != nil {
		return nil, apperrors.NewNotFound("application not found by client_id: " + clientID)
	}

	return toDetail(application), nil
}

// GetApplicationPublicByClientID is the read-only projection used by the
// anonymous discovery endpoint. It shares GetByClientID's tenant-free lookup
// (the client ID is the selector by design) but returns ApplicationPublicDetail,
// so SAML, callback and transport configuration never reach a pre-login caller.
func (s *Service) GetApplicationPublicByClientID(ctx context.Context, clientID string) (*ApplicationPublicDetail, error) {
	// INC-BE-05：匿名预登录发现没有用户租户上下文，fail-closed RLS 下必须用
	// System 只读定位；提升收敛在 app 层这个只读用例内，transport 不提升。
	application, err := s.UOW.Applications().GetByClientID(tenantctx.System(ctx), clientID)
	if err != nil {
		return nil, apperrors.NewNotFound("application not found by client_id: " + clientID)
	}

	return toPublicDetail(application), nil
}
