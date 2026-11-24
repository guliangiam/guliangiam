package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
)

// ImpersonationSessionResolver resolves an opaque impersonation token into a
// trusted session projection. It is an optional capability of the auth service.
type ImpersonationSessionResolver interface {
	ResolveImpersonationSession(ctx context.Context, rawToken string) (*appauth.ImpersonationSession, error)
}

// SessionContextMiddleware 解析 session context（repository + header 两种来源）。
// Phase C-Step 1 新增 principalResolver：可选，传入时从 platform_admins 表解析 PrincipalType。
// 未传入时 session.PrincipalType 默认空字符串 → authorization.go 的 principalModel() 会走 defaultRBACModel（普通 user）。
func SessionContextMiddleware(handler Handler, jwtIssuer TokenVerifier, principalResolver ...scopeauthorizer.PrincipalTypeResolver) func(http.Handler) http.Handler {
	var resolver scopeauthorizer.PrincipalTypeResolver
	if len(principalResolver) > 0 {
		resolver = principalResolver[0]
	}
	// Opaque impersonation resolver. Without a cookie session it is the
	// fallback after JWT validation fails; with a cookie session it may take
	// priority only when the token is bound to that cookie's user
	// (impersonationOverrideForCookie, 裁定 Q2-A).
	var impersonation ImpersonationSessionResolver
	if handler.ImpersonationResolver != nil {
		impersonation = handler.ImpersonationResolver
	} else if handler.Service != nil {
		impersonation = handler.Service
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 会话只来自仓库：历史 HMAC 签名 Cookie（不落库、因此不受撤销约束）的解码兼容
			// 已随 OPEN-API-19 一并删除，不再存在「无对应会话行即成立」的来源。
			if session, ok := sessionContextFromRepository(r, handler); ok {
				// 显式携带且绑定到本 cookie 用户的 imp token 优先于 cookie 身份；
				// 无关/未绑定 token 一律忽略，cookie 会话原样保留。
				if override, bound := impersonationOverrideForCookie(r, session.UserID, impersonation); bound {
					session = override
				}
				fillPrincipalType(r.Context(), &session, resolver)
				next.ServeHTTP(w, r.WithContext(WithSessionContext(r.Context(), session)))
				return
			}

			if session, ok := sessionContextFromHeaders(r, jwtIssuer, impersonation); ok {
				fillPrincipalType(r.Context(), &session, resolver)
				next.ServeHTTP(w, r.WithContext(WithSessionContext(r.Context(), session)))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// impersonationSessionContext projects a validated imp token into a regular
// user session. PrincipalType is intentionally left empty here: it is resolved
// from the target user afterwards, so impersonation never inherits the
// caller's platform-admin model.
func impersonationSessionContext(session *appauth.ImpersonationSession, accessToken string) SessionContext {
	return SessionContext{
		UserID:           session.UserID,
		Username:         session.Username,
		OrganizationID:   session.OrganizationID,
		OrganizationSlug: session.OrganizationSlug,
		AccessToken:      accessToken,
		Source:           "impersonation",
		AuthTime:         time.Now(),
	}
}

// impersonationOverrideForCookie lets an explicitly presented imp token take
// priority over the repository cookie session, but only when the token's
// metadata.impersonated_by equals the cookie session's user (裁定 Q2-A 绑定校验).
// A foreign, unbound, invalid or absent token never shadows the cookie session.
func impersonationOverrideForCookie(r *http.Request, cookieUserID string, resolver ImpersonationSessionResolver) (SessionContext, bool) {
	cookieUserID = strings.TrimSpace(cookieUserID)
	if resolver == nil || cookieUserID == "" {
		return SessionContext{}, false
	}
	accessToken := accessTokenFromRequest(r)
	if accessToken == "" {
		return SessionContext{}, false
	}
	session, err := resolver.ResolveImpersonationSession(r.Context(), accessToken)
	if err != nil || session == nil {
		return SessionContext{}, false
	}
	if session.ImpersonatedBy == "" || session.ImpersonatedBy != cookieUserID {
		return SessionContext{}, false
	}
	return impersonationSessionContext(session, accessToken), true
}

// fillPrincipalType 如果有 resolver 则填充 session.PrincipalType。
// 用 session.UserID 查 platform_admins.user_id（不是 session row ID）。
// 未查到 platform_admin 记录 → 空字符串（authorization.go principalModel() 会走 defaultRBACModel）。
func fillPrincipalType(ctx context.Context, session *SessionContext, resolver scopeauthorizer.PrincipalTypeResolver) {
	if resolver == nil || session == nil || session.UserID == "" {
		return
	}
	session.PrincipalType = resolver.Resolve(ctx, session.UserID)
}
