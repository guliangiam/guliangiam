package auth

import (
	"context"
	"net/http"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// IsPlatformAdmin reports whether the verified session belongs to an active
// platform administrator. PrincipalType is resolved from platform_admins by
// scopeauthorizer.PrincipalTypeResolver during SessionContextMiddleware, so
// request input can never promote a caller here.
func IsPlatformAdmin(ctx context.Context) bool {
	session, ok := SessionContextFromContext(ctx)
	if !ok {
		return false
	}
	return session.PrincipalType == string(scopeauthorizer.PrincipalPlatformAdmin)
}

// RequirePlatformAdmin makes a handler fail closed unless the verified session
// is a platform administrator. The /api/v1/global/* listings are cross-tenant
// by design: PostgreSQL RLS escalates platform admins to super_admin, while
// MySQL has no RLS and would otherwise return every tenant's rows.
func RequirePlatformAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsPlatformAdmin(r.Context()) {
			route.WriteError(w, r, apperrors.NewForbidden("platform administrator privileges required"))
			return
		}
		next(w, r)
	}
}
