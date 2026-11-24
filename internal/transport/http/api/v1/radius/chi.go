package radius

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChiRoutes registers RADIUS server management routes.
func RegisterChiRoutes(router chi.Router, handler *Handler) {
	// Start the RADIUS server.
	router.Post("/api/v1/radius/start", func(w http.ResponseWriter, r *http.Request) {
		var req StartServerRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.StartServer(r.Context(), req.Addr, req.Secret); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, map[string]any{
			"status": "started",
			"addr":   req.Addr,
		})
	})

	// Stop the RADIUS server.
	router.Post("/api/v1/radius/stop", func(w http.ResponseWriter, r *http.Request) {
		if err := handler.StopServer(); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, map[string]any{
			"status": "stopped",
		})
	})

	// Get RADIUS server status.
	router.Get("/api/v1/radius/status", func(w http.ResponseWriter, r *http.Request) {
		status := handler.GetStatus()
		route.WriteJSON(w, http.StatusOK, status)
	})
}

// StartServerRequest contains the parameters for starting the RADIUS server.
type StartServerRequest struct {
	Addr   string `json:"addr,omitempty"`   // Address to listen on, default ":1812"
	Secret string `json:"secret,omitempty"` // Shared secret for RADIUS authentication
}
