package mcpservers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	appmcpservers "github.com/guliangiam/guliangiam/internal/app/mcpservers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/mcp-servers", func(w http.ResponseWriter, r *http.Request) {
		request := ListMcpServersRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: auth.TrustedOrganizationID(r, r.URL.Query().Get("organizationId")),
			Keyword:        r.URL.Query().Get("keyword"),
		}
		if v := r.URL.Query().Get("isEnabled"); v != "" {
			if parsed, err := strconv.ParseBool(v); err == nil {
				request.IsEnabled = &parsed
			}
		}
		response, err := handler.List(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/mcp-servers", func(w http.ResponseWriter, r *http.Request) {
		var request CreateMcpServerRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		// Tenant comes from the authenticated scope, never from the payload.
		request.OrganizationID = auth.TrustedOrganizationID(r, request.OrganizationID)
		if err := handler.Create(r.Context(), request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, presenter.NewActionResponse("created", "resource created"))
	})

	router.Get("/api/v1/mcp-servers/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.Get(r.Context(), organizationID, GetMcpServerRequest{ID: chi.URLParam(r, "id")})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Patch("/api/v1/mcp-servers/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request PatchMcpServerRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.Patch(r.Context(), organizationID, chi.URLParam(r, "id"), request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
	})

	router.Delete("/api/v1/mcp-servers/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.Delete(r.Context(), organizationID, chi.URLParam(r, "id")); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})

	// Online registry: list MCP servers from a remote registry
	router.Get("/api/v1/mcp-servers/online", func(w http.ResponseWriter, r *http.Request) {
		registryURL := r.URL.Query().Get("registryUrl")
		response, err := handler.GetOnlineServers(r.Context(), registryURL)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	// Intranet scan: discover MCP servers on the local network
	router.Post("/api/v1/mcp-servers/sync-intranet", func(w http.ResponseWriter, r *http.Request) {
		var req SyncIntranetServersHTTPRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.SyncIntranetServers(r.Context(), appmcpservers.SyncIntranetServersRequest{
			// Tenant comes from the authenticated scope, never from the payload.
			OrganizationID: auth.TrustedOrganizationID(r, req.OrganizationID),
			CIDRs:          req.CIDRs,
			Ports:          req.Ports,
			Concurrency:    req.Concurrency,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})

	// Tool sync: discover tools from a specific MCP server
	router.Post("/api/v1/mcp-servers/{id}/sync-tools", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.SyncTools(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})

	// Access token: issue a short-lived token for the authenticated session user.
	// The subject is never caller-selected: a query userId is only tolerated while
	// it names the same session user, so a mcpservers:read holder cannot mint a
	// token for a colleague. No server/administrator delegation path exists in
	// this build; adding one would require a separate higher privilege point.
	router.Get("/api/v1/mcp-servers/{id}/access-token", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		userID, err := trustedSessionUserID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if requested := strings.TrimSpace(r.URL.Query().Get("userId")); requested != "" && requested != userID {
			route.WriteError(w, r, apperrors.NewForbidden("cannot mint an MCP access token for another user"))
			return
		}
		result, err := handler.GetAccessToken(r.Context(), organizationID, chi.URLParam(r, "id"), userID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})

	// Proxy: forward tool calls to an MCP server. The tenant is resolved before
	// the body is decoded; a path orgSlug is only honoured for a platform scope.
	// The caller identity comes from the validated session, never from the query.
	router.Post("/api/v1/mcp-servers/{orgSlug}/{serverName}/proxy", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgIDFor(r, chi.URLParam(r, "orgSlug"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		userID, err := trustedSessionUserID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req ProxyServerHTTPRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.ProxyRequest(r.Context(), organizationID, chi.URLParam(r, "serverName"), req.ToolName, req.Arguments, userID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
}

// trustedSessionUserID returns the local user of the authenticated session.
// Self-service MCP operations (token minting, tool invocation) must act as this
// user; query/body identifiers are not authoritative. A federated subject that
// has no local user row is rejected rather than mapped to an arbitrary account.
func trustedSessionUserID(r *http.Request) (string, error) {
	session, ok := auth.SessionContextFromContext(r.Context())
	if !ok {
		return "", apperrors.NewUnauthorized("authenticated session is required")
	}
	userID := strings.TrimSpace(session.UserID)
	if userID == "" {
		return "", apperrors.NewForbidden("a local session user is required")
	}
	return userID, nil
}

// trustedOrgID resolves the tenant for by-ID routes from the query
// organizationId.
func trustedOrgID(r *http.Request) (string, error) {
	return trustedOrgIDFor(r, r.URL.Query().Get("organizationId"))
}

// trustedOrgIDFor resolves the tenant for by-ID routes. The requested value
// (query organizationId or path orgSlug) is only honoured for a platform
// scope; a session-bound caller can never widen its own tenant by naming
// another tenant in the request.
func trustedOrgIDFor(r *http.Request, requested string) (string, error) {
	organizationID := auth.TrustedOrganizationID(r, requested)
	if organizationID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	return organizationID, nil
}
