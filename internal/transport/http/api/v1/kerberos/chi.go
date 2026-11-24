package kerberos

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	platformhttp "github.com/guliangiam/guliangiam/internal/platform/http"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChiRoutes registers Kerberos authentication routes.
func RegisterChiRoutes(router chi.Router, handler *Handler) {
	// SPNEGO/Kerberos negotiate endpoint (browser-initiated SSO)
	router.Get("/api/v1/auth/kerberos/negotiate", func(w http.ResponseWriter, r *http.Request) {
		handler.negotiate(w, r)
	})

	// Kerberos login via SPNEGO token (API client)
	router.Post("/api/v1/auth/kerberos/login", func(w http.ResponseWriter, r *http.Request) {
		handler.login(w, r)
	})
}

type kerberosLoginRequest struct {
	OrganizationSlug string   `json:"organizationSlug"`
	SPNEGOToken      string   `json:"spnegoToken"`
	Realm            string   `json:"realm"`
	Service          string   `json:"service"`
	ApplicationID    string   `json:"applicationId"`
	ResponseType     string   `json:"responseType"`
	RedirectURI      string   `json:"redirectUri"`
	State            string   `json:"state"`
	Nonce            string   `json:"nonce"`
	Scope            []string `json:"scope"`
}

func (h Handler) login(w http.ResponseWriter, r *http.Request) {
	var req kerberosLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		route.WriteOAuthError(w, http.StatusBadRequest, route.OAuthErrInvalidRequest, "invalid request body")
		return
	}

	result, err := h.AuthService.KerberosLogin(r.Context(), appauth.KerberosLoginCommand{
		OrganizationSlug: req.OrganizationSlug,
		SPNEGOToken:      req.SPNEGOToken,
		Realm:            req.Realm,
		Service:          req.Service,
		ApplicationID:    req.ApplicationID,
		IPAddress:        r.RemoteAddr,
		UserAgent:        r.UserAgent(),
		ResponseType:     req.ResponseType,
		RedirectURI:      req.RedirectURI,
		State:            req.State,
		Nonce:            req.Nonce,
		Scope:            req.Scope,
	})
	if err != nil {
		// A failed SPNEGO ticket means the presented credential did not
		// authenticate, which is RFC 6749 §5.2 `invalid_client`. The body must
		// not echo err.Error(): the hand-written http.Error this replaced did
		// exactly that, and also skipped the protocol envelope entirely.
		route.WriteOAuthError(w, http.StatusUnauthorized, route.OAuthErrInvalidClient,
			route.SafeProtocolDescription(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h Handler) negotiate(w http.ResponseWriter, r *http.Request) {
	// Extract SPNEGO token from Authorization header
	authHeader := r.Header.Get("Authorization")
	tokenStr, isNegotiate := platformhttp.ParseNegotiateHeader(authHeader)

	if !isNegotiate || tokenStr == "" {
		// No Negotiate header: send 401 with WWW-Authenticate to initiate SPNEGO
		w.Header().Set("WWW-Authenticate", platformhttp.BuildNegotiateChallenge(""))
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Validate the SPNEGO token
	orgSlug := r.URL.Query().Get("organization")
	if orgSlug == "" {
		orgSlug = "default"
	}

	result, err := h.AuthService.KerberosLogin(r.Context(), appauth.KerberosLoginCommand{
		OrganizationSlug: orgSlug,
		SPNEGOToken:      tokenStr,
		IPAddress:        r.RemoteAddr,
		UserAgent:        r.UserAgent(),
	})
	if err != nil {
		// Keep the SPNEGO challenge: this is not a Bearer endpoint, so the
		// challenge must stay `Negotiate` — route.WriteProtocolError would
		// overwrite it with `Bearer` and break the browser's retry. The body
		// still gets the protocol envelope, which http.Error did not produce.
		w.Header().Set("WWW-Authenticate", platformhttp.BuildNegotiateChallenge(""))
		route.WriteOAuthError(w, http.StatusUnauthorized, route.OAuthErrInvalidClient,
			route.SafeProtocolDescription(err))
		return
	}

	// On success, return the login result with session info
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
