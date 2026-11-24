package batch

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Post("/api/v1/batch/users", func(w http.ResponseWriter, r *http.Request) {
		handleBatch(w, r, handler)
	})

	router.Post("/api/v1/batch/organizations", func(w http.ResponseWriter, r *http.Request) {
		handleBatch(w, r, handler)
	})

	router.Post("/api/v1/batch/applications", func(w http.ResponseWriter, r *http.Request) {
		handleBatch(w, r, handler)
	})
}

func handleBatch(w http.ResponseWriter, r *http.Request, handler Handler) {
	var req BatchRequest
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}

	response, err := handler.HandleBatch(r.Context(), req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	route.WriteJSON(w, http.StatusOK, response)
}
