package saml

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

const (
	// maxSPACSResponseBytes is checked before the assertion reaches the SAML
	// parser; the root router already caps the whole request body at 1MB.
	maxSPACSResponseBytes = 512 * 1024
	// maxSPACSRelayStateBytes matches infra/saml.RelayStateSigner's envelope cap.
	maxSPACSRelayStateBytes = 4096
)

// SPACSCompleter consumes a browser ACS callback. It exists so the transport
// layer can be tested without constructing the full auth unit of work.
type SPACSCompleter interface {
	CompleteSAMLSocialLogin(ctx context.Context, command appauth.CompleteSAMLSocialLoginCommand) (*appauth.SocialLoginResult, error)
}

// RegisterPublicChiRoutes mounts the only unauthenticated SAML SP endpoint: the
// IdP's browser form POST target. It must not sit behind session/CSRF middleware
// (there is no session yet and the POST is cross-site by design), so bootstrap
// calls this from the root public group.
func RegisterPublicChiRoutes(router chi.Router, handler *Handler) {
	router.With(route.WithSurface(route.SurfaceProtocol)).
		Post("/api/v1/auth/saml/acs/{organizationSlug}/{providerId}", func(w http.ResponseWriter, r *http.Request) {
			handler.handleSPACS(w, r)
		})
}

func (h Handler) handleSPACS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	orgSlug := strings.TrimSpace(chi.URLParam(r, "organizationSlug"))
	providerID := strings.TrimSpace(chi.URLParam(r, "providerId"))
	if orgSlug == "" || providerID == "" {
		route.WriteError(w, r, apperrors.NewInvalidArgument("organizationSlug and providerId are required"))
		return
	}
	if err := r.ParseForm(); err != nil {
		route.WriteError(w, r, errBadRequest("invalid form data"))
		return
	}

	samlResponse := r.PostFormValue("SAMLResponse")
	relayState := r.PostFormValue("RelayState")
	if strings.TrimSpace(samlResponse) == "" || strings.TrimSpace(relayState) == "" {
		route.WriteError(w, r, apperrors.NewInvalidArgument("SAMLResponse and RelayState are required"))
		return
	}
	if len(samlResponse) > maxSPACSResponseBytes {
		route.WriteError(w, r, apperrors.NewInvalidArgument("SAMLResponse exceeds the maximum size"))
		return
	}
	if len(relayState) > maxSPACSRelayStateBytes {
		route.WriteError(w, r, apperrors.NewInvalidArgument("RelayState exceeds the maximum size"))
		return
	}

	result, err := h.completeSPACS(r.Context(), appauth.CompleteSAMLSocialLoginCommand{
		OrganizationSlug:  orgSlug,
		ProviderID:        providerID,
		SAMLResponse:      samlResponse,
		RelayState:        relayState,
		IPAddress:         util.ClientIP(r),
		UserAgent:         strings.TrimSpace(r.UserAgent()),
		PreviousSessionID: authtransport.ExtractPreviousSessionIDFromRequest(r),
	})
	if err != nil {
		mw.RecordLoginError(orgSlug, "saml", "acs_failed")
		if h.AuthService != nil {
			meta := route.RequestMetadataFrom(r.Context())
			_ = h.AuthService.RecordSAMLAcsFailure(r.Context(), appauth.SAMLAcsFailureCommand{
				OrganizationSlug: orgSlug,
				ProviderID:       providerID,
				Reason:           "acs_failed",
				IPAddress:        util.ClientIP(r),
				UserAgent:        strings.TrimSpace(r.UserAgent()),
				RequestID:        meta.RequestID,
				TraceID:          meta.TraceID,
			})
		}
		route.WriteError(w, r, err)
		return
	}

	mw.RecordLoginAttempt(orgSlug, "saml", "success")
	if result != nil && strings.TrimSpace(result.SessionID) != "" {
		authtransport.SetOpaqueSessionCookie(w, result.SessionID)
	}

	target := "/"
	if result != nil {
		if redirect := strings.TrimSpace(result.Response.RedirectTo); redirect != "" {
			target = redirect
		}
	}
	// Location is a header value; reject a CR/LF smuggled through the redirect
	// URI instead of letting the header writer reject the whole response.
	if strings.ContainsAny(target, "\r\n") {
		route.WriteError(w, r, apperrors.NewInternal("invalid redirect target"))
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (h Handler) completeSPACS(ctx context.Context, command appauth.CompleteSAMLSocialLoginCommand) (*appauth.SocialLoginResult, error) {
	if h.SPACSCompleter != nil {
		return h.SPACSCompleter.CompleteSAMLSocialLogin(ctx, command)
	}
	if h.AuthService == nil {
		return nil, apperrors.NewInternal("auth service is not configured")
	}
	return h.AuthService.CompleteSAMLSocialLogin(ctx, command)
}
