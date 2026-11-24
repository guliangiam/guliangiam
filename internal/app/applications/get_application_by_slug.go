package applications

import (
	"context"
	"errors"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// GetApplicationByOrgAndSlug looks up an application by organization slug and application slug.
// This supports the "owner/name" lookup pattern used by the legacy console and SDKs.
// The route is anonymous pre-login discovery, so it returns the same public
// projection as by-client-id: SAML material, callback allowlists, transport
// configuration and token policy never reach an unauthenticated caller.
func (s *Service) GetApplicationByOrgAndSlug(ctx context.Context, orgSlug, appSlug string) (*ApplicationPublicDetail, error) {
	// orgSlug/appSlug 来自 query（applications/resolve，public）；WT 内先用公开投影 GetBySlugPublic 解析组织，再按 org.ID 查 Applications。公开投影不读取凭据/树字段，且不依赖租户 RLS 上下文。
	orgSlug = strings.TrimSpace(orgSlug)
	appSlug = strings.TrimSpace(appSlug)
	if orgSlug == "" || appSlug == "" {
		return nil, apperrors.NewInvalidArgument("organization slug and application slug are required")
	}

	var detail *ApplicationPublicDetail
	var lookupErr error
	// 匿名公共发现读：未登录请求的租户作用域会回退到 built-in（TenantFilterMiddleware），
	// 导致 ownerSlug 指向其他组织时应用查询被 RLS 遮蔽（404）。这里与
	// internal/app/auth/client_credentials.go 的匿名发现读采用同一模式：
	// 事务作用域提升为 System，组织 ID 来自公开投影（DB 可信来源），
	// 应用查询仍按 organization_id 显式过滤，事务保持只读。
	err := s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		org, err := tx.Organizations().GetBySlugPublic(ctx, orgSlug)
		if err != nil {
			if !isNotFoundError(err) {
				return err
			}
			lookupErr = apperrors.NewNotFound("organization not found: " + orgSlug)
			return lookupErr
		}

		application, err := tx.Applications().GetByOrganizationAndSlug(ctx, org.ID, appSlug)
		if err != nil {
			if !isNotFoundError(err) {
				return err
			}
			lookupErr = apperrors.NewNotFound("application not found: " + orgSlug + "/" + appSlug)
			return lookupErr
		}

		detail = toPublicDetail(application)
		return nil
	})
	if err != nil {
		if lookupErr != nil {
			return nil, lookupErr
		}
		return nil, err
	}
	return detail, nil
}

func isNotFoundError(err error) bool {
	var appErr *apperrors.Error
	return errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound
}
