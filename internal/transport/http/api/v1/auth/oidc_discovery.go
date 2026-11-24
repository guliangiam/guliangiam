package auth

import (
	"net/http"

	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// OIDCDiscoveryHandler returns an http.HandlerFunc that serves the OIDC Discovery
// document at GET /.well-known/openid-configuration. The issuer URL is derived
// from the ISSUER_URL environment variable, or falls back to the request's Host
// header (respecting X-Forwarded-Proto and X-Forwarded-Host).
func OIDCDiscoveryHandler(handler Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issuerURL := platformconfig.IssuerURL(r)
		response, err := handler.Service.GetOIDCDiscovery(r.Context(), issuerURL)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	}
}
