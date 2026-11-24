package users

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func registerContactVerificationRoutes(router chi.Router, handler Handler) {
	router.Post("/api/v1/users/{id}/contact-verification/send", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request ContactVerificationSendRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		ticket, err := handler.SendContactVerification(r.Context(), organizationID, chi.URLParam(r, "id"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, ticket)
	})

	router.Post("/api/v1/users/{id}/contact-verification/verify", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request ContactVerificationVerifyRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.VerifyContactVerification(r.Context(), organizationID, chi.URLParam(r, "id"), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("verified", "contact verified"))
	})
}
