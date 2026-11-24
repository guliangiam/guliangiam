package i18n

import "github.com/go-chi/chi/v5"

// RegisterChiRoutes registers i18n routes on the given Chi router.
func RegisterChiRoutes(r chi.Router, h *Handler) {
	r.Route("/api/v1/i18n", func(r chi.Router) {
		r.Get("/languages", h.ListLanguages)
		r.Get("/translations/{language}", h.GetTranslations)
		r.Put("/translations/{language}", h.UpdateTranslation)
		r.Get("/translate", h.Translate)
	})
}
