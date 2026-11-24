package webauthn

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
	apiauth "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

// Handler handles WebAuthn protocol endpoints.
type Handler struct {
	AuthService *appauth.Service
}

// rateLimitKey 是 WebAuthn 路由的限流主体：已登录会话按用户 ID 计数，匿名请求
// 回落到 ClientIP（socket 地址；只有 socket peer 是可信代理时才采信转发头）。
// 不使用可伪造的裸请求头做键。
func rateLimitKey(r *http.Request) string {
	if session, ok := apiauth.SessionContextFromContext(r.Context()); ok {
		if userID := strings.TrimSpace(session.UserID); userID != "" {
			return "user:" + userID
		}
	}
	return "ip:" + util.ClientIP(r)
}

// RegisterChiRoutes registers WebAuthn protocol routes.
//
// 每个路由族各自持有一个认证级限流桶（AuthRateLimit）：注册、认证、凭据管理互不
// 占用额度，且 begin/finish 都受限。挂载位置在 SessionContext 之后，因此已登录
// 请求按用户维度、匿名请求按 IP 维度计数（见 rateLimitKey）。
func RegisterChiRoutes(router chi.Router, handler *Handler) {
	router.Group(func(r chi.Router) {
		r.Use(mw.RateLimit(mw.AuthRateLimit, mw.WithKeyFunc(rateLimitKey)))
		registerCredentialRoutes(r, handler)
	})

	router.Group(func(r chi.Router) {
		r.Use(mw.RateLimit(mw.AuthRateLimit, mw.WithKeyFunc(rateLimitKey)))
		registerRegistrationRoutes(r, handler)
	})

	router.Group(func(r chi.Router) {
		r.Use(mw.RateLimit(mw.AuthRateLimit, mw.WithKeyFunc(rateLimitKey)))
		registerAuthenticationRoutes(r, handler)
	})
}

func registerCredentialRoutes(router chi.Router, handler *Handler) {
	// Self-service credential management (session-bound; ownership enforced in app layer).
	router.Get("/api/v1/auth/webauthn/credentials", func(w http.ResponseWriter, r *http.Request) {
		session, ok := apiauth.SessionContextFromContext(r.Context())
		if !ok {
			route.WriteError(w, r, apperrors.NewUnauthorized("a session is required"))
			return
		}
		credentials, err := handler.AuthService.ListWebAuthnCredentials(r.Context(), session.UserID, session.OrganizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, credentials)
	})

	router.Delete("/api/v1/auth/webauthn/credentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		session, ok := apiauth.SessionContextFromContext(r.Context())
		if !ok {
			route.WriteError(w, r, apperrors.NewUnauthorized("a session is required"))
			return
		}
		if err := handler.AuthService.RevokeWebAuthnCredential(r.Context(), session.UserID, session.OrganizationID, chi.URLParam(r, "id")); err != nil {
			route.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func registerRegistrationRoutes(router chi.Router, handler *Handler) {
	// Registration ceremony.
	router.Post("/api/v1/auth/webauthn/registration/begin", func(w http.ResponseWriter, r *http.Request) {
		var request WebAuthnRegistrationBeginRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		command := appauth.WebAuthnRegistrationBeginCommand{
			OrganizationSlug: request.OrganizationSlug,
			Username:         request.Username,
		}
		if session, ok := apiauth.SessionContextFromContext(r.Context()); ok {
			command.SessionUserID = session.UserID
			command.SessionOrganizationID = session.OrganizationID
		}
		result, err := handler.AuthService.WebAuthnRegistrationBegin(r.Context(), command)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})

	router.Post("/api/v1/auth/webauthn/registration/finish", func(w http.ResponseWriter, r *http.Request) {
		var request WebAuthnRegistrationFinishRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		command := appauth.WebAuthnRegistrationFinishCommand{
			OrganizationSlug:  request.OrganizationSlug,
			Username:          request.Username,
			ChallengeID:       request.ChallengeID,
			Challenge:         request.Challenge,
			CredentialID:      request.CredentialID,
			PublicKey:         request.PublicKey,
			ClientDataJSON:    request.ClientDataJSON,
			AuthenticatorData: request.AuthenticatorData,
			AttestationObject: request.AttestationObject,
			AttestationType:   request.AttestationType,
			Transports:        request.Transports,
			PreviousSessionID: apiauth.ExtractPreviousSessionIDFromRequest(r),
		}
		if session, ok := apiauth.SessionContextFromContext(r.Context()); ok {
			command.SessionUserID = session.UserID
			command.SessionOrganizationID = session.OrganizationID
		}
		result, sessionID, err := handler.AuthService.WebAuthnRegistrationFinish(r.Context(), command)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if sessionID != "" {
			apiauth.SetOpaqueSessionCookie(w, sessionID)
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
}

func registerAuthenticationRoutes(router chi.Router, handler *Handler) {
	// Authentication ceremony.
	router.Post("/api/v1/auth/webauthn/authentication/begin", func(w http.ResponseWriter, r *http.Request) {
		var request WebAuthnAuthenticationBeginRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		result, err := handler.AuthService.WebAuthnAuthenticationBegin(r.Context(), appauth.WebAuthnAuthenticationBeginCommand{
			OrganizationSlug: request.OrganizationSlug,
			Username:         request.Username,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})

	router.Post("/api/v1/auth/webauthn/authentication/finish", func(w http.ResponseWriter, r *http.Request) {
		var request WebAuthnAuthenticationFinishRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		result, sessionID, err := handler.AuthService.WebAuthnAuthenticationFinish(r.Context(), appauth.WebAuthnAuthenticationFinishCommand{
			OrganizationSlug:  request.OrganizationSlug,
			Username:          request.Username,
			ChallengeID:       request.ChallengeID,
			Challenge:         request.Challenge,
			CredentialID:      request.CredentialID,
			Signature:         request.Signature,
			AuthenticatorData: request.AuthenticatorData,
			ClientDataJSON:    request.ClientDataJSON,
			ApplicationID:     request.ApplicationID,
			ResponseType:      request.ResponseType,
			RedirectURI:       request.RedirectURI,
			State:             request.State,
			Nonce:             request.Nonce,
			Scope:             request.Scope,
			PreviousSessionID: apiauth.ExtractPreviousSessionIDFromRequest(r),
			// 会话元数据由服务端从连接与请求头解析，不接受客户端自报。
			IPAddress: util.ClientIP(r),
			UserAgent: r.UserAgent(),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		// 会话在 finish 签发：登录页是匿名调用方，这里种下 cookie 才算登录完成。
		if sessionID != "" {
			apiauth.SetOpaqueSessionCookie(w, sessionID)
		}

		route.WriteJSON(w, http.StatusOK, result.Response)
	})
}

// Request types.

type WebAuthnRegistrationBeginRequest struct {
	OrganizationSlug string `json:"organizationSlug"`
	Username         string `json:"username"`
}

type WebAuthnRegistrationFinishRequest struct {
	OrganizationSlug  string   `json:"organizationSlug"`
	Username          string   `json:"username"`
	ChallengeID       string   `json:"challengeId"`
	Challenge         string   `json:"challenge"`
	CredentialID      string   `json:"credentialId"`
	PublicKey         string   `json:"publicKey"`
	ClientDataJSON    string   `json:"clientDataJSON"`
	AuthenticatorData string   `json:"authenticatorData"`
	AttestationObject string   `json:"attestationObject"`
	AttestationType   string   `json:"attestationType"`
	Transports        []string `json:"transports,omitempty"`
}

type WebAuthnAuthenticationBeginRequest struct {
	OrganizationSlug string `json:"organizationSlug"`
	Username         string `json:"username"`
}

type WebAuthnAuthenticationFinishRequest struct {
	OrganizationSlug  string `json:"organizationSlug"`
	Username          string `json:"username"`
	ChallengeID       string `json:"challengeId"`
	Challenge         string `json:"challenge"`
	CredentialID      string `json:"credentialId"`
	Signature         string `json:"signature"`
	AuthenticatorData string `json:"authenticatorData"`
	ClientDataJSON    string `json:"clientDataJSON"`

	// 协议上下文（可选）：控制台在 OAuth2/OIDC/CAS/SAML 入口发起 WebAuthn 登录时
	// 携带，使 finish 与密码登录共用同一条后处理路径（《开发设计》§45.3）。
	ApplicationID string   `json:"applicationId,omitempty"`
	ResponseType  string   `json:"responseType,omitempty"`
	RedirectURI   string   `json:"redirectUri,omitempty"`
	State         string   `json:"state,omitempty"`
	Nonce         string   `json:"nonce,omitempty"`
	Scope         []string `json:"scope,omitempty"`
}
