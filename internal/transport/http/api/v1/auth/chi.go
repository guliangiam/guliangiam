package auth

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChiRoutes mounts the /api/v1/auth/* tree. That tree carries two of
// the three API faces at once (手册 4.1.3): the authentication face (login,
// signup, MFA, captcha, session and social endpoints) and the protocol
// endpoints a standard client calls (authorize, tokens:*, device/*, oauth2/*,
// cas/*, saml/*, kerberos/*).
//
// Each face is its own router group, so the rate-limit policy and the surface
// tag are bound per face instead of per route: metrics, audit rows and OpenAPI
// tags inherit the face from route.WithSurface and cannot drift from the
// surface contract gate.
func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Route("/api/v1/auth", func(r chi.Router) {
		// Session-context reads (contact / account / permissions) are mounted first
		// with the general API limit: the console calls them on every bootstrap, so
		// the credential bucket below (10/min, burst 5) would 429 a user seconds
		// after a successful login. Credential writes keep the strict bucket.
		r.Group(func(s chi.Router) {
			s.Use(route.WithSurface(route.SurfaceAuthentication))
			s.Use(mw.RateLimit(mw.DefaultAPIRateLimit))
			registerSessionReadRoutes(s, handler)
		})

		// Authentication face: page and session endpoints, rate limit
		// RATE_LIMIT_AUTH_RATE/BURST (手册 4.1.3).
		r.Group(func(a chi.Router) {
			a.Use(route.WithSurface(route.SurfaceAuthentication))
			a.Use(mw.RateLimit(mw.AuthRateLimit))
			registerAuthenticationRoutes(a, handler)
		})

		// Protocol face: prefix family (5) of the six families in 手册 4.1.3,
		// rate limit RATE_LIMIT_TOKEN_RATE/BURST.
		r.Group(func(p chi.Router) {
			p.Use(route.WithSurface(route.SurfaceProtocol))
			p.Use(mw.RateLimit(mw.TokenEndpointRateLimit))
			registerProtocolRoutes(p, handler)
		})
	})
}

func registerAuthenticationRoutes(router chi.Router, handler Handler) {
	registerSharedRoutes(router, handler)
	registerPasswordRoutes(router, handler)
	registerQrcodeRoutes(router, handler)
	registerFaceidRoutes(router, handler)
	registerWeb3Routes(router, handler)
	registerSocialRoutes(router, handler)
	registerMfaRoutes(router, handler)
	registerTrustedDeviceRoutes(router, handler)
	registerRememberMeRoutes(router, handler)
	registerStepUpRoutes(router, handler)
	registerSsoRoutes(router, handler)
	registerFrontChannelRoutes(router, handler)
}

func accessTokenFromRequest(r *http.Request) string {
	if token := strings.TrimSpace(r.Header.Get("X-Access-Token")); token != "" {
		return token
	}

	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if authorization == "" {
		return ""
	}

	if strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return strings.TrimSpace(authorization[7:])
	}

	return authorization
}

// sessionSubjectFromRequest returns the caller's session subject (username +
// organization UUID) when a session is present, and empty strings otherwise.
//
// 扫码/确认的身份只能来自会话：cookie 会话与 Bearer JWT 都会先经
// SessionContextMiddleware 注入 context，故统一从这里取，而不是只认 JWT 的
// sessionContextFromHeaders。
func sessionSubjectFromRequest(r *http.Request) (string, string) {
	session, ok := SessionContextFromContext(r.Context())
	if !ok {
		return "", ""
	}
	return strings.TrimSpace(session.Username), strings.TrimSpace(session.OrganizationID)
}

// sessionContextFromOpaqueImpersonation turns a validated "imp" token into a
// regular user session so impersonation traffic reaches the protected chain.
func sessionContextFromOpaqueImpersonation(r *http.Request, accessToken string, resolver ImpersonationSessionResolver) (SessionContext, bool) {
	if resolver == nil || strings.TrimSpace(accessToken) == "" {
		return SessionContext{}, false
	}
	session, err := resolver.ResolveImpersonationSession(r.Context(), accessToken)
	if err != nil || session == nil {
		return SessionContext{}, false
	}
	return impersonationSessionContext(session, accessToken), true
}

func sessionContextFromHeaders(r *http.Request, jwtIssuer TokenVerifier, impersonationResolvers ...ImpersonationSessionResolver) (SessionContext, bool) {
	var impersonation ImpersonationSessionResolver
	if len(impersonationResolvers) > 0 {
		impersonation = impersonationResolvers[0]
	}
	accessToken := accessTokenFromRequest(r)
	if accessToken == "" {
		return SessionContext{}, false
	}

	// Validate the JWT token to get the true identity claims.
	// OTel span: child of TracingMiddleware top-level HTTP span.
	var claims map[string]any
	{
		span := mw.RecordAuthSpan(r.Context(), "jwt.validate")
		var err error
		claims, err = jwtIssuer.ValidateToken(accessToken)
		span.End()
		if err != nil {
			// Opaque impersonation tokens are not JWTs. Try them only after JWT
			// validation fails. The repository-cookie path ran before this
			// function (a cookie-bound imp token already took priority there);
			// reaching here means no cookie session exists.
			if session, ok := sessionContextFromOpaqueImpersonation(r, accessToken, impersonation); ok {
				return session, true
			}
			return SessionContext{}, false
		}
	}

	// Extract verified identity from the JWT claims, ignoring untrusted headers.
	subject, _ := claims["sub"].(string)
	username, _ := claims["username"].(string)
	orgID, _ := claims["organization_id"].(string)
	if username == "" || orgID == "" {
		// Fallback: try subject claim for username.
		if username == "" {
			username = subject
		}
		if username == "" || orgID == "" {
			return SessionContext{}, false
		}
	}

	// 只有本地用户 UUID 才能进入 UserID 及后续平台管理员解析。
	// 联邦/workload 的外部 subject 不能触发本地 platform_admins 查询。
	localUserID := ""
	if parsed, err := uuid.Parse(subject); err == nil {
		localUserID = parsed.String()
	}

	// Use the organization_id from the token to look up the slug.
	// The header values are ignored — identity comes solely from the signed JWT.
	return SessionContext{
		UserID:           localUserID,
		OrganizationID:   orgID,
		OrganizationSlug: orgID, // Will be resolved by handler if slug is needed
		Username:         username,
		AccessToken:      accessToken,
		Source:           "header",
	}, true
}
