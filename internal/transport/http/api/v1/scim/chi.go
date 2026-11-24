package scim

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	appscim "github.com/guliangiam/guliangiam/internal/app/scim"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// BearerTokenAuth middleware validates the Bearer token against application client credentials.
// The token is expected in the Authorization header as "Bearer <token>".
// It checks the token against all applications' client secrets in the specified organization
// and, on success, binds that organization to the request context as the trusted tenant scope.
// Handlers read the scope instead of re-reading the request's organizationId.
func BearerTokenAuth(appService appapplications.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeSCIMError(w, http.StatusUnauthorized, "Authorization header is required")
				return
			}

			// Parse Bearer token.
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeSCIMError(w, http.StatusUnauthorized, "Invalid authorization header format, expected: Bearer <token>")
				return
			}

			token := strings.TrimSpace(parts[1])
			if token == "" {
				writeSCIMError(w, http.StatusUnauthorized, "Bearer token is empty")
				return
			}

			// The organization ID is required to scope the token validation.
			orgID := r.URL.Query().Get("organizationId")
			if orgID == "" {
				orgID = r.Header.Get("X-Organization-Id")
			}

			if orgID == "" {
				// Without an organization context, we cannot validate the token.
				writeSCIMError(w, http.StatusUnauthorized, "organizationId is required for SCIM authentication")
				return
			}

			// 校验前先绑定租户作用域：ValidateSCIMBearerToken 需要读取该组织下的应用凭据，
			// 而连接池默认 fail-closed（app.current_organization_id 为空时 RLS 拒绝所有行），
			// 不先绑定就会查出 0 个应用，导致任何 token 都被判为 invalid。
			// 此处的 orgID 仅作为查询范围，真正的授权由下面的 token 校验结果决定。
			ctx := tenantctx.WithTenantScope(r.Context(), orgID, "", tenantctx.TenantFilterStrict)

			// Validate the bearer token against the organization's application client credentials.
			valid, err := appService.ValidateSCIMBearerToken(ctx, orgID, token)
			if err != nil {
				writeSCIMError(w, http.StatusUnauthorized, "failed to validate credentials")
				return
			}

			if !valid {
				writeSCIMError(w, http.StatusUnauthorized, "Invalid bearer token")
				return
			}

			// Bind the organization that was just validated to the request context:
			// handlers must not re-read a request-sourced organizationId, otherwise
			// validation scope and execution scope are two independent inputs.
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ChiRouter returns a Chi router with SCIM v2 endpoints.
func ChiRouter(h *Handler, appService appapplications.Service) chi.Router {
	r := chi.NewRouter()

	// 本路由以 Mount("/") 挂到主路由（bootstrap/chi.go），会兜住所有未匹配
	// 路径。Bearer 认证必须限定在 /scim/v2 子树内：若全局 r.Use，跳过平台
	// 认证的公开前缀（/.well-known/*、/oauth2/* 等）下的未知路径会落入
	// SCIM 中间件，返回 SCIM 格式 401 而不是主路由的 404。
	r.Route("/scim/v2", func(r chi.Router) {
		r.Use(BearerTokenAuth(appService))

		// SCIM v2 User endpoints
		r.Route("/Users", func(r chi.Router) {
			r.Get("/", h.ListUsers)
			r.Post("/", h.CreateUser)
			r.Get("/{id}", h.GetUser)
			r.Put("/{id}", h.UpdateUser)
			r.Patch("/{id}", h.PatchUser)
			r.Delete("/{id}", h.DeleteUser)
		})

		// SCIM v2 Group endpoints
		r.Route("/Groups", func(r chi.Router) {
			r.Get("/", h.ListGroups)
			r.Post("/", h.CreateGroup)
			r.Get("/{id}", h.GetGroup)
			r.Put("/{id}", h.UpdateGroup)
			r.Patch("/{id}", h.PatchGroup)
			r.Delete("/{id}", h.DeleteGroup)
		})

		// SCIM v2 Discovery endpoints — Bearer-authenticated like the data family.
		// The unauthenticated discovery document lives at /.well-known/scim-configuration.
		r.Get("/ServiceProviderConfig", appscim.HandleServiceProviderConfig)
		r.Get("/ResourceTypes", appscim.HandleResourceTypes)
		r.Get("/Schemas", appscim.HandleSchemas)
	})

	return r
}
