package auth

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// registerProtocolRoutes registers the protocol-face endpoints that live under
// /api/v1/auth/* (手册 4.1.3 前缀家族 (5)): the RFC 6749/7523/8693/7662 token
// endpoints, RFC 9449 DPoP validation, OIDC back-channel logout, the consent
// endpoints, the device grant, and the CAS / SAML / Kerberos protocol handlers.
//
// RegisterChiRoutes mounts them inside the /api/v1/auth/* route, so the session
// and authorization chain the bootstrap router puts on the authentication face
// applies here as well; only the rate-limit group and the surface tag differ.
func registerProtocolRoutes(router chi.Router, handler Handler) {

	// RFC 7523 JWT-Bearer Grant Type
	router.Post("/tokens/jwt-bearer", func(w http.ResponseWriter, r *http.Request) {
		var request JwtBearerTokenRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.GetJwtBearerToken(r.Context(), request.toCommand(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// RFC 8693 Token Exchange
	router.Post("/tokens/token-exchange", func(w http.ResponseWriter, r *http.Request) {
		var request TokenExchangeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.GetTokenExchangeToken(r.Context(), request.toCommand())
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// RFC 7662 Token Introspection
	router.Post("/tokens/introspect", func(w http.ResponseWriter, r *http.Request) {
		var request TokenIntrospectionRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.IntrospectToken(r.Context(), request.toCommand())
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// OIDC Back-Channel Logout
	router.Post("/backchannel-logout", func(w http.ResponseWriter, r *http.Request) {
		var request BackchannelLogoutRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.SendBackchannelLogout(r.Context(), request.toCommand(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// RFC 9449 DPoP Proof Validation
	router.Post("/dpop/validate", func(w http.ResponseWriter, r *http.Request) {
		var request DPoPProofValidationRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.ValidateDPoPProof(r.Context(), request.toCommand())
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// RFC 9449 §4.2.2 DPoP Nonce Challenge
	// A client that receives "DPoP nonce is required" from the validator should
	// call this endpoint, embed the returned nonce in its next proof's `nonce`
	// claim, and retry the failed request. The nonce is one-time-use.
	router.Post("/dpop/nonce", func(w http.ResponseWriter, r *http.Request) {
		result, err := handler.GenerateDPoPNonce(r.Context())
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		w.Header().Set("DPoP-Nonce", result.Nonce)
		route.WriteJSON(w, http.StatusOK, map[string]any{
			"nonce":      result.Nonce,
			"expires_in": result.ExpiresIn,
		})
	})

	// Note: /signup and /password/reset routes are registered in RegisterChiRoutes
	// with dedicated stricter rate limits, not here.

	// OAuth Consent
	router.Post("/consents/grant", func(w http.ResponseWriter, r *http.Request) {
		var request GrantConsentRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.GrantConsent(r.Context(), request.toCommand(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/consents/check", func(w http.ResponseWriter, r *http.Request) {
		var request EvaluateConsentRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.EvaluateConsent(r.Context(), request.toCommand(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/consents/revoke", func(w http.ResponseWriter, r *http.Request) {
		var request RevokeConsentRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.RevokeConsent(r.Context(), request.toCommand(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// OAuth2 Authorization
	router.Post("/authorize", func(w http.ResponseWriter, r *http.Request) {
		var request AuthorizeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.Authorize(r.Context(), request.toCommand(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// Front-channel authorization endpoint (RFC 6749 §4.1.1). A browser hits
	// this with query parameters; we answer with redirects so the user agent
	// can finish the flow. Unauthenticated callers are sent to the console
	// login page with the original query preserved (no redirect_uri trust is
	// needed for that hop), consent goes to the console consent page, and a
	// successful authorize redirects to the client's registered redirect_uri.
	router.Get("/authorize", func(w http.ResponseWriter, r *http.Request) {
		// Fail-closed login hop: actor identity comes only from the session
		// middleware, never from headers.
		if _, ok := SessionContextFromContext(r.Context()); !ok {
			target := "/login/oauth/authorize"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusFound)
			return
		}

		request := authorizeRequestFromQuery(r)
		response, err := handler.Authorize(r.Context(), request.toCommand(r))
		if err != nil {
			// Do not redirect errors to an unvalidated redirect_uri (open
			// redirect). The console renders the JSON error instead.
			route.WriteError(w, r, err)
			return
		}

		if response.Code != nil && response.Code.RedirectTo != "" {
			http.Redirect(w, r, response.Code.RedirectTo, http.StatusFound)
			return
		}
		if response.Status == "consent_required" && response.Evaluation != nil {
			http.Redirect(w, r, authorizeConsentRedirect(response.Evaluation.ApplicationSlug, r), http.StatusFound)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	// OAuth2 Token Exchange
	router.Post("/tokens/exchange", func(w http.ResponseWriter, r *http.Request) {
		request, err := decodeExchangeAuthorizationCodeRequest(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		if request.isClientCredentialsGrant() {
			// System 只读定位与租户回钉由 app 层两段式事务负责（INC-BE-05），
			// transport 不做请求级 RLS 提升。
			response, err := handler.ExchangeClientCredentials(r.Context(), request.toClientCredentialsCommand(r))
			if err != nil {
				route.WriteError(w, r, err)
				return
			}
			route.WriteJSON(w, http.StatusOK, response)
			return
		}

		if request.isRefreshTokenGrant() {
			response, err := handler.ExchangeRefreshToken(r.Context(), request.toRefreshTokenCommand(r))
			if err != nil {
				route.WriteError(w, r, err)
				return
			}
			route.WriteJSON(w, http.StatusOK, response)
			return
		}

		response, err := handler.ExchangeAuthorizationCode(r.Context(), request.toCommand(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// Client Authentication
	router.Post("/client/authenticate", func(w http.ResponseWriter, r *http.Request) {
		var request ClientAuthRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.AuthenticateClient(r.Context(), request.toCommand(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// SAML IdP Metadata endpoint
	router.Get("/saml/metadata/{orgSlug}/{appSlug}", func(w http.ResponseWriter, r *http.Request) {
		orgSlug := chi.URLParam(r, "orgSlug")
		appSlug := chi.URLParam(r, "appSlug")

		metadata, err := handler.Service.GenerateSamlMetadataBySlug(r.Context(), orgSlug, appSlug)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/samlmetadata+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(metadata))
	})

	// ─────────────────────────────────────────────
	// Client Credentials Grant
	// ─────────────────────────────────────────────

	router.Post("/tokens/client-credentials", func(w http.ResponseWriter, r *http.Request) {
		var request ClientCredentialsRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.ExchangeClientCredentials(r.Context(), request.toCommand(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// ─────────────────────────────────────────────
	// Device Authorization (RFC 8628)
	// ─────────────────────────────────────────────

	router.Post("/device/authorize", func(w http.ResponseWriter, r *http.Request) {
		var request DeviceAuthorizeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		result, err := handler.DeviceAuthorize(r.Context(), request.toCommand(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})

	router.Post("/device/token", func(w http.ResponseWriter, r *http.Request) {
		var request DeviceTokenRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.DeviceToken(r.Context(), request.toCommand())
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/device/verify", func(w http.ResponseWriter, r *http.Request) {
		var request DeviceVerifyRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.DeviceVerify(r.Context(), request.toCommand(r)); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "operation succeeded"))
	})

	router.Post("/device/cancel", func(w http.ResponseWriter, r *http.Request) {
		var request DeviceCancelRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.CancelDeviceAuthorization(r.Context(), request.toCommand()); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "device authorization cancelled"))
	})

	// ─────────────────────────────────────────────
	// CAS Protocol (1.0/2.0/3.0)
	// ─────────────────────────────────────────────

	router.Get("/cas/login", handler.CasLoginRedirect)
	router.Get("/cas/serviceValidate", handler.CasServiceValidate)
	router.Get("/cas/proxyValidate", handler.CasProxyValidate)
	router.Get("/cas/proxy", handler.CasProxy)
	router.Post("/cas/logout", handler.CasLogout)

	// ─────────────────────────────────────────────
	// Kerberos Login
	// ─────────────────────────────────────────────

	router.Get("/kerberos/login", func(w http.ResponseWriter, r *http.Request) {
		// Check if Kerberos is configured for the organization.
		orgSlug := r.URL.Query().Get("organization")
		if orgSlug == "" {
			orgSlug = "default"
		}

		// Kerberos authentication is always available via the database-backed
		// ticket store. The Service handles SPNEGO validation internally.

		// Return 401 with WWW-Authenticate: Negotiate header to initiate
		// SPNEGO/Kerberos browser authentication.
		w.Header().Set("WWW-Authenticate", "Negotiate")
		w.WriteHeader(http.StatusUnauthorized)
	})

	// ─────────────────────────────────────────────
	// RFC 7009 Token Revocation
	// ─────────────────────────────────────────────

	router.Post("/oauth2/revoke", func(w http.ResponseWriter, r *http.Request) {
		command := decodeTokenRevocationRequest(r)

		if err := handler.RevokeToken(r.Context(), command); err != nil {
			route.WriteError(w, r, err)
			return
		}

		// RFC 7009: always return 200 OK
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "operation succeeded"))
	})

	// ─────────────────────────────────────────────
	// RFC 7662 Token Introspection (RFC-compliant)
	// ─────────────────────────────────────────────

	router.Post("/oauth2/introspect", func(w http.ResponseWriter, r *http.Request) {
		command := decodeTokenIntrospectionRFCRequest(r)

		response, err := handler.IntrospectTokenRFC(r.Context(), command)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})
}

// authorizeRequestFromQuery maps RFC 6749 front-channel query parameters onto
// the JSON request DTO. Both the standard snake_case names and the console's
// camelCase aliases are accepted; camelCase wins when both are present.
func authorizeRequestFromQuery(r *http.Request) AuthorizeRequest {
	q := r.URL.Query()
	value := func(standard, alias string) string {
		if v := strings.TrimSpace(q.Get(alias)); v != "" {
			return v
		}
		return strings.TrimSpace(q.Get(standard))
	}
	request := AuthorizeRequest{
		ClientID:        value("client_id", "clientId"),
		RedirectURI:     value("redirect_uri", "redirectUri"),
		ResponseType:    value("response_type", "responseType"),
		State:           value("state", "state"),
		Nonce:           value("nonce", "nonce"),
		CodeChallenge:   value("code_challenge", "codeChallenge"),
		ChallengeMethod: value("code_challenge_method", "codeChallengeMethod"),
		Resource:        value("resource", "resource"),
	}
	if scope := value("scope", "requestedScopes"); scope != "" {
		request.RequestedScopes = strings.Fields(scope)
	}
	return request
}

// authorizeConsentRedirect builds the console consent URL, preserving every
// original query parameter exactly once so the POST that follows can complete
// the same authorization request (PKCE/state/nonce included).
func authorizeConsentRedirect(applicationSlug string, r *http.Request) string {
	target := "/consent/" + url.PathEscape(applicationSlug)
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	return target
}
