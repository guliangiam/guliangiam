package payments

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChannelConfigChiRoutes 注册渠道配置管理 HTTP 路由 (design doc G6 §P2).
// 与主 Handler / ReconHandler 分离 — ChannelConfigHandler 只依赖 apppayments.ChannelConfigService.
func RegisterChannelConfigChiRoutes(router chi.Router, handler ChannelConfigHandler) {
	// List
	router.Get("/api/v1/pay-channel-configs", func(w http.ResponseWriter, r *http.Request) {
		resp, err := handler.List(r.Context(), ListChannelConfigsRequest{
			OrganizationID: auth.TrustedOrganizationID(r, r.URL.Query().Get("organizationId")),
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", platformpagination.DefaultPageSize),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	// Create
	router.Post("/api/v1/pay-channel-configs", func(w http.ResponseWriter, r *http.Request) {
		var req CreateChannelConfigRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		// Tenant comes from the authenticated scope, never from the payload.
		req.OrganizationID = auth.TrustedOrganizationID(r, req.OrganizationID)
		cfg, err := handler.Create(r.Context(), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, cfg)
	})

	// Get by ID
	router.Get("/api/v1/pay-channel-configs/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		cfg, err := handler.Get(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, cfg)
	})

	// State transition
	router.Post("/api/v1/pay-channel-configs/{id}:transition", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req ChannelConfigTransitionRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		cfg, err := handler.Transition(r.Context(), organizationID, chi.URLParam(r, "id"), req.Status)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, cfg)
	})
}
