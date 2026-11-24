// tenant_scope.go — AI Gateway 管理面的可信租户裁决。
//
// 管理面 handler 一律通过 trustedOrgID 取租户：TenantFilter 注入的租户作用域
// 是唯一权威，query 里的 organizationId 只是平台管理员的显式跨租户 target。
// 空租户必须 fail-fast 成 400——在无 RLS 的方言（MySQL）上，空
// organization_id 会退化成“全部租户”。
package aigateway

import (
	"net/http"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

// trustedOrgID resolves the tenant a management request may act on. A
// caller-supplied organizationId is honoured only for platform administrators;
// otherwise the authenticated tenant scope (or session org) wins.
func trustedOrgID(r *http.Request, requested string) (string, error) {
	orgID := authtransport.TrustedOrganizationID(r, requested)
	if orgID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	return orgID, nil
}
