package tokens

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request := ListTokensRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: organizationID,
			UserID:         r.URL.Query().Get("userId"),
			ApplicationID:  r.URL.Query().Get("applicationId"),
			ActiveOnly:     route.QueryOptionalBool(r, "activeOnly"),
			Keyword:        r.URL.Query().Get("keyword"),
		}

		response, err := handler.List(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		var request CreateTokenRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		organizationID, err := trustedOrgID(r, request.OrganizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request.OrganizationID = organizationID

		response, err := handler.Create(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteCreated(w, "/api/v1/tokens/"+response.Token.ID, response)
	})

	router.Post("/api/v1/tokens/revoke", func(w http.ResponseWriter, r *http.Request) {
		var request RevokeTokensRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		// The trusted tenant is a scope constraint, not a filter default: an
		// unfiltered batch revoke must stay a client error instead of silently
		// becoming "revoke every token in my tenant".
		if strings.TrimSpace(request.OrganizationID) == "" &&
			strings.TrimSpace(request.UserID) == "" &&
			strings.TrimSpace(request.ApplicationID) == "" {
			route.WriteError(w, r, apperrors.NewInvalidArgument("at least one filter is required to revoke tokens"))
			return
		}
		organizationID, err := trustedOrgID(r, request.OrganizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request.OrganizationID = organizationID

		response, err := handler.RevokeMany(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/tokens/introspect", func(w http.ResponseWriter, r *http.Request) {
		var request IntrospectTokenRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request.OrganizationID = organizationID

		response, err := handler.Introspect(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/tokens/refresh", func(w http.ResponseWriter, r *http.Request) {
		var request RefreshTokenRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request.OrganizationID = organizationID

		response, err := handler.Refresh(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/tokens/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.Get(r.Context(), GetTokenRequest{
			ID:             chi.URLParam(r, "id"),
			OrganizationID: organizationID,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		// The app repository read is scoped; this is a defence-in-depth
		// assertion so a future regression cannot return a foreign row.
		if !tokenBelongsToOrg(response, organizationID) {
			route.WriteError(w, r, apperrors.NewNotFound("token not found"))
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/tokens/{id}/revoke", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request := GetTokenRequest{ID: chi.URLParam(r, "id"), OrganizationID: organizationID}
		if err := handler.Revoke(r.Context(), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("revoked", "resource revoked"))
	})

	router.Delete("/api/v1/tokens/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request := GetTokenRequest{ID: chi.URLParam(r, "id"), OrganizationID: organizationID}
		if err := handler.Delete(r.Context(), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})
}

// tokenBelongsToOrg reports whether the fetched token is inside the trusted
// tenant. A missing body is treated as not found so the check fails closed.
// The repository already binds the read to the trusted tenant; this assertion
// only guards against an app-layer regression.
func tokenBelongsToOrg(token *TokenResponse, organizationID string) bool {
	return token != nil && organizationID != "" && token.OrganizationID == organizationID
}

// trustedOrgID resolves the tenant a request may act on. A caller-supplied
// organizationId is honoured only for platform administrators; otherwise the
// authenticated tenant scope wins. An empty result is a client error: on
// dialects without RLS an empty organization_id degrades to "all tenants".
func trustedOrgID(r *http.Request, requested string) (string, error) {
	orgID := authtransport.TrustedOrganizationID(r, requested)
	if orgID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	return orgID, nil
}
