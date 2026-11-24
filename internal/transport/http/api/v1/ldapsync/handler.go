package ldapsync

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	appldapsync "github.com/guliangiam/guliangiam/internal/app/ldapsync"
	platformerrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

type Handler struct {
	Service *appldapsync.Service
}

func NewHandler(service *appldapsync.Service) *Handler {
	return &Handler{Service: service}
}

// RegisterRoutes registers LDAP sync config routes.
func RegisterRoutes(router chi.Router, handler *Handler) {
	router.Route("/api/v1/ldap-sync-configs", func(r chi.Router) {
		r.Get("/", handler.List)
		r.Post("/", handler.Create)
		r.Get("/{id}", handler.Get)
		r.Patch("/{id}", handler.Update)
		r.Delete("/{id}", handler.Delete)
		r.Post("/{id}/sync", handler.TriggerSync)
		r.Get("/{id}/logs", handler.GetSyncLogs)
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID := authtransport.TrustedOrganizationID(r, r.URL.Query().Get("organizationId"))
	if orgID == "" {
		route.WriteError(w, r, platformerrors.NewInvalidArgument("organizationId is required"))
		return
	}

	configs, err := h.Service.ListSyncConfigs(r.Context(), orgID)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	route.WriteJSON(w, http.StatusOK, map[string]any{
		"items": configs,
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	id := chi.URLParam(r, "id")
	config, err := h.Service.GetSyncConfig(r.Context(), organizationID, id)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, config)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateSyncConfigRequest
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	// The body's organizationId only requests a target tenant; the
	// authenticated scope decides which tenant is actually written.
	req.OrganizationID = authtransport.TrustedOrganizationID(r, req.OrganizationID)
	if req.OrganizationID == "" {
		route.WriteError(w, r, platformerrors.NewInvalidArgument("organizationId is required"))
		return
	}

	config := req.toCommand()
	id, err := h.Service.CreateSyncConfig(r.Context(), config)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	route.WriteCreated(w, "/api/v1/ldap-sync-configs/"+id, map[string]any{"id": id})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	id := chi.URLParam(r, "id")

	var req PatchSyncConfigRequest
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}

	if err := h.Service.PatchSyncConfig(r.Context(), organizationID, id, req.toPatch()); err != nil {
		route.WriteError(w, r, err)
		return
	}

	route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "operation succeeded"))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.Service.DeleteSyncConfig(r.Context(), organizationID, id); err != nil {
		route.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TriggerSync(w http.ResponseWriter, r *http.Request) {
	organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	id := chi.URLParam(r, "id")
	result, err := h.Service.TriggerSync(r.Context(), organizationID, id)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) GetSyncLogs(w http.ResponseWriter, r *http.Request) {
	organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	id := chi.URLParam(r, "id")

	logs, err := h.Service.GetSyncLogs(r.Context(), organizationID, id, 20)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	route.WriteJSON(w, http.StatusOK, map[string]any{"items": logs})
}

// trustedOrgID resolves the tenant a request may act on. A caller-supplied
// organizationId is honoured only for platform administrators; otherwise the
// authenticated tenant scope wins. An empty result is a client error: on
// dialects without RLS an empty organization_id degrades to "all tenants".
func trustedOrgID(r *http.Request, requested string) (string, error) {
	orgID := authtransport.TrustedOrganizationID(r, requested)
	if orgID == "" {
		return "", platformerrors.NewInvalidArgument("organizationId is required")
	}
	return orgID, nil
}

// Request types

type CreateSyncConfigRequest struct {
	OrganizationID      string `json:"organizationId"`
	LdapID              string `json:"ldapId"`
	SyncIntervalMinutes int    `json:"syncIntervalMinutes"`
	BaseDN              string `json:"baseDn"`
	UserFilter          string `json:"userFilter"`
	GroupFilter         string `json:"groupFilter"`
	Enabled             bool   `json:"enabled"`
}

func (r *CreateSyncConfigRequest) toCommand() appldapsync.SyncConfigInput {
	return appldapsync.SyncConfigInput{
		OrganizationID:      r.OrganizationID,
		LdapID:              r.LdapID,
		SyncIntervalMinutes: r.SyncIntervalMinutes,
		BaseDN:              r.BaseDN,
		UserFilter:          r.UserFilter,
		GroupFilter:         r.GroupFilter,
		Enabled:             r.Enabled,
	}
}

type PatchSyncConfigRequest struct {
	SyncIntervalMinutes *int    `json:"syncIntervalMinutes"`
	BaseDN              *string `json:"baseDn"`
	UserFilter          *string `json:"userFilter"`
	GroupFilter         *string `json:"groupFilter"`
	Enabled             *bool   `json:"enabled"`
}

func (r *PatchSyncConfigRequest) toPatch() appldapsync.SyncConfigPatch {
	return appldapsync.SyncConfigPatch{
		SyncIntervalMinutes: r.SyncIntervalMinutes,
		BaseDN:              r.BaseDN,
		UserFilter:          r.UserFilter,
		GroupFilter:         r.GroupFilter,
		Enabled:             r.Enabled,
	}
}
