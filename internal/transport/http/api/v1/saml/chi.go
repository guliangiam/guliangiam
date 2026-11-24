package saml

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// Handler handles SAML 2.0 protocol endpoints.
type Handler struct {
	AuthService *appauth.Service
	// SPACSCompleter is the transport seam for the public browser ACS. Nil means
	// the real AuthService is used; tests inject a fake without a full UoW.
	SPACSCompleter SPACSCompleter
}

// RegisterChiRoutes registers SAML 2.0 protocol routes.
func RegisterChiRoutes(router chi.Router, handler *Handler) {
	// ── SP-initiated SSO: 为外部 SP 生成 AuthnRequest（OPEN-FE-07）──
	router.Post("/api/v1/auth/saml/authn-request/{organizationSlug}/{applicationSlug}", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			route.WriteError(w, r, errBadRequest("invalid form data"))
			return
		}
		orgSlug := chi.URLParam(r, "organizationSlug")
		appSlug := chi.URLParam(r, "applicationSlug")
		spEntityID := r.FormValue("spEntityId")
		spACSURL := r.FormValue("spAcsUrl")
		relayState := r.FormValue("relayState")

		result, err := handler.AuthService.BuildSPAuthnRequest(r.Context(), appauth.BuildSPAuthnRequestCommand{
			OrganizationSlug: orgSlug,
			ApplicationSlug:  appSlug,
			SPEntityID:       spEntityID,
			SPACSURL:         spACSURL,
			RelayState:       relayState,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})

	// SP-initiated SSO: redirect binding (existing endpoint).
	router.Get("/api/v1/auth/saml/redirect/{organizationSlug}/{applicationSlug}", func(w http.ResponseWriter, r *http.Request) {
		samlRequest := r.URL.Query().Get("SAMLRequest")
		relayState := r.URL.Query().Get("RelayState")

		result, err := handler.AuthService.HandleSAMLRedirect(r.Context(), appauth.SAMLRequestCommand{
			OrganizationSlug: chi.URLParam(r, "organizationSlug"),
			ApplicationSlug:  chi.URLParam(r, "applicationSlug"),
			SAMLRequest:      samlRequest,
			RelayState:       relayState,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		// Redirect to frontend SAML login page.
		orgSlug := chi.URLParam(r, "organizationSlug")
		appSlug := chi.URLParam(r, "applicationSlug")
		redirectURL := fmt.Sprintf("/login/saml/authorize/%s/%s?samlRequest=%s&relayState=%s",
			orgSlug, appSlug,
			urlEncode(result.SAMLRequest),
			urlEncode(result.RelayState),
		)

		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	})

	// IdP metadata endpoint (existing endpoint).
	router.Get("/api/v1/auth/saml/metadata/{organizationSlug}/{applicationId}", func(w http.ResponseWriter, r *http.Request) {
		metadata, err := handler.AuthService.GetSAMLMetadata(r.Context(),
			chi.URLParam(r, "organizationSlug"),
			chi.URLParam(r, "applicationId"),
		)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/samlmetadata+xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(metadata)
	})

	// SAML ACS (Assertion Consumer Service) - generates SAML response (existing endpoint).
	router.Post("/api/v1/auth/saml/acs", func(w http.ResponseWriter, r *http.Request) {
		samlRequest := r.FormValue("SAMLRequest")
		relayState := r.FormValue("RelayState")
		applicationID := r.FormValue("applicationId")

		if strings.TrimSpace(samlRequest) == "" {
			route.WriteError(w, r, errBadRequest("SAMLRequest is required"))
			return
		}

		// Fail-close：身份只从 session context 取，不接受表单里的 username /
		// organizationSlug（见 sessionFromRequest）。
		session, ok := sessionFromRequest(r)
		if !ok {
			route.WriteError(w, r, apperrors.NewUnauthorized("authentication required"))
			return
		}

		result, err := handler.AuthService.GenerateSAMLResponse(r.Context(), appauth.SAMLResponseCommand{
			OrganizationSlug: session.OrganizationSlug,
			Username:         session.Username,
			ApplicationID:    applicationID,
			SAMLRequest:      samlRequest,
			RelayState:       relayState,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		// Return the SAML response as JSON for the frontend to handle.
		route.WriteJSON(w, http.StatusOK, result)
	})

	// --- New SAML IdP endpoints ---

	// SAML metadata endpoint (new path with applicationId only).
	router.Get("/api/v1/saml/metadata/{applicationId}", func(w http.ResponseWriter, r *http.Request) {
		applicationID := chi.URLParam(r, "applicationId")
		if applicationID == "" {
			route.WriteError(w, r, errBadRequest("applicationId is required"))
			return
		}

		metadata, err := handler.AuthService.GenerateSamlMetadata(r.Context(), applicationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/samlmetadata+xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(metadata))
	})

	// SAML SSO POST binding.
	router.Post("/api/v1/saml/sso/{applicationId}", func(w http.ResponseWriter, r *http.Request) {
		handler.handleSamlSSO(w, r)
	})

	// SAML SSO GET binding.
	router.Get("/api/v1/saml/sso/{applicationId}", func(w http.ResponseWriter, r *http.Request) {
		handler.handleSamlSSO(w, r)
	})

	// ─────────────────────────────────────────────
	// SAML Single Logout (SLO)
	// ─────────────────────────────────────────────

	// SP-initiated SLO: redirect to IdP's SLO endpoint.
	router.Get("/api/v1/saml/slo/{applicationId}", func(w http.ResponseWriter, r *http.Request) {
		applicationID := chi.URLParam(r, "applicationId")
		if applicationID == "" {
			route.WriteError(w, r, errBadRequest("applicationId is required"))
			return
		}

		sessionCtx, ok := sessionFromRequest(r)
		if !ok {
			route.WriteError(w, r, errBadRequest("authentication required for SLO"))
			return
		}

		result, err := handler.AuthService.HandleSPInitiatedSLO(r.Context(), appauth.SAMLSLOCommand{
			ApplicationID: applicationID,
			Username:      sessionCtx.Username,
			SessionIndex:  "", // Session index derived from NameID-based session lookup
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		http.Redirect(w, r, result.RedirectURL, http.StatusFound)
	})

	// IdP-initiated SLO: receive LogoutRequest, terminate session, return LogoutResponse.
	router.Post("/api/v1/saml/slo", func(w http.ResponseWriter, r *http.Request) {
		samlRequest := r.FormValue("SAMLRequest")
		if samlRequest == "" {
			samlRequest = r.URL.Query().Get("SAMLRequest")
		}
		if samlRequest == "" {
			route.WriteError(w, r, errBadRequest("SAMLRequest is required"))
			return
		}

		logoutResponse, err := handler.AuthService.HandleIdPInitiatedSLO(r.Context(), samlRequest)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		// Return the LogoutResponse as a SAML redirect or POST form.
		relayState := r.FormValue("RelayState")
		if relayState == "" {
			relayState = r.URL.Query().Get("RelayState")
		}

		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(logoutResponse))
	})

	// IdP-initiated SLO via GET (redirect binding).
	router.Get("/api/v1/saml/slo", func(w http.ResponseWriter, r *http.Request) {
		samlRequest := r.URL.Query().Get("SAMLRequest")
		if samlRequest == "" {
			route.WriteError(w, r, errBadRequest("SAMLRequest is required"))
			return
		}

		logoutResponse, err := handler.AuthService.HandleIdPInitiatedSLO(r.Context(), samlRequest)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(logoutResponse))
	})
}

// handleSamlSSO handles SAML SSO requests for both GET and POST bindings.
func (h Handler) handleSamlSSO(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	if applicationID == "" {
		route.WriteError(w, r, errBadRequest("applicationId is required"))
		return
	}

	var samlRequest string
	var relayState string

	switch r.Method {
	case http.MethodGet:
		samlRequest = r.URL.Query().Get("SAMLRequest")
		relayState = r.URL.Query().Get("RelayState")
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			route.WriteError(w, r, errBadRequest("invalid form data"))
			return
		}
		samlRequest = r.FormValue("SAMLRequest")
		relayState = r.FormValue("RelayState")
	}

	if strings.TrimSpace(samlRequest) == "" {
		route.WriteError(w, r, errBadRequest("SAMLRequest is required"))
		return
	}

	// Fail-fast on malformed requests before any session work.
	if _, err := h.AuthService.ParseSamlRequest(r.Context(), samlRequest); err != nil {
		route.WriteError(w, r, err)
		return
	}

	// For unauthenticated users, redirect to the login page.
	redirectURL := fmt.Sprintf("/login/saml/authorize?applicationId=%s&samlRequest=%s&relayState=%s",
		urlEncode(applicationID),
		urlEncode(samlRequest),
		urlEncode(relayState),
	)

	// Check if the user is already authenticated (session context).
	if session, ok := sessionFromRequest(r); ok {
		// User is authenticated, generate SAML response.
		result, err := h.AuthService.GenerateSAMLResponse(r.Context(), appauth.SAMLResponseCommand{
			OrganizationSlug: session.OrganizationSlug,
			Username:         session.Username,
			ApplicationID:    applicationID,
			SAMLRequest:      samlRequest,
			RelayState:       relayState,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		// POST the SAML response to the whitelist-validated ACS URL resolved by
		// the application layer — never to the raw SP-requested destination.
		postSamlResponse(w, result.RedirectURL, result.SAMLResponse, relayState)
		return
	}

	// Redirect to the login page.
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// postSamlResponse renders an HTML form that auto-submits the SAML response to
// the SP. samlResponse is already base64-encoded by the application layer and
// is posted verbatim — encoding it again here would corrupt the assertion.
func postSamlResponse(w http.ResponseWriter, destination, samlResponse, relayState string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>SAML Response</title></head>
<body onload="document.forms[0].submit()">
<form method="POST" action="%s">
<input type="hidden" name="SAMLResponse" value="%s" />
<input type="hidden" name="RelayState" value="%s" />
</form>
</body>
</html>`, escapeHTML(destination), escapeHTML(samlResponse), escapeHTML(relayState))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// sessionFromRequest returns the session that auth.SessionContextMiddleware
// attached to the request — and nothing else.
//
// Fail-close（架构 §8.8 / PEP fail-close 审计 2026-09-24）：身份**只能**从
// session context 取。此处曾 fallback 读 X-Organization-Slug / X-Username /
// X-Access-Token 请求头，而 /api/v1/saml/* 在 PEP 白名单内（authorization.go
// isTrulyPublicEndpoint），于是未认证请求只要带上这三个头就能以任意用户身份取得
// 签名 SAML 断言 —— 认证绕过。auth/request.go 已按同一审计结论移除该模式，本文件
// 是当时的漏网点。
func sessionFromRequest(r *http.Request) (authtransport.SessionContext, bool) {
	session, ok := authtransport.SessionContextFromContext(r.Context())
	// 中间件在解析不出会话时不注入 context；即便注入了空结构体，也不得视为已认证 ——
	// 否则 SAMLResponseCommand 会带着空 organizationSlug/username 继续走签发路径。
	if !ok || session.OrganizationSlug == "" || session.Username == "" {
		return authtransport.SessionContext{}, false
	}
	return session, true
}

func urlEncode(s string) string {
	return strings.ReplaceAll(s, "+", "%2B")
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// errBadRequest builds the 400 an invalid SAML request must produce. It used to
// return a package-local type that carried only a message, and the protocol face
// renders through route.WriteError → OAuthErrorFrom: an error that is neither an
// *apperrors.Error nor an HTTPStatus() implementer falls through to 500
// `server_error`. Asking the application namespace gives the intended 400 +
// `invalid_request` for free (手册 4.1.3 三面表 / 错误码注册表 §2).
func errBadRequest(msg string) error {
	return apperrors.NewInvalidArgument(msg)
}
