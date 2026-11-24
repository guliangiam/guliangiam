package dcr

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	// RFC 7591: Register a new OAuth2 client (no authentication required for initial registration).
	router.Post("/oauth2/register", func(w http.ResponseWriter, r *http.Request) {
		var request RegisterClientRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.RegisterClient(r.Context(), request)
		if err != nil {
			// RFC 7591 §3.2.2: registration failures use the OAuth error body
			// (invalid_client_metadata / invalid_redirect_uri …), not the console
			// application envelope.
			route.WriteProtocolError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusCreated, response)
	})

	// RFC 7591: Read a registered client. Requires the registration_access_token
	// issued at registration; without it the caller learns nothing about the
	// client (设计报告 14.28).
	router.Get("/oauth2/register/{clientID}", func(w http.ResponseWriter, r *http.Request) {
		clientID := chi.URLParam(r, "clientID")
		response, err := handler.GetClient(r.Context(), GetClientRequest{
			ClientID:                clientID,
			RegistrationAccessToken: registrationAccessTokenFromRequest(r, ""),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// RFC 7591: Update a registered client. Requires registration_access_token.
	router.Put("/oauth2/register/{clientID}", func(w http.ResponseWriter, r *http.Request) {
		clientID := chi.URLParam(r, "clientID")
		var request UpdateClientRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.UpdateClient(r.Context(), clientID, registrationAccessTokenFromRequest(r, request.RegistrationAccessToken), request)
		if err != nil {
			// RFC 7592 §3.1: metadata updates answer with the OAuth error body too.
			route.WriteProtocolError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// RFC 7591: Delete a registered client. Requires registration_access_token.
	router.Delete("/oauth2/register/{clientID}", func(w http.ResponseWriter, r *http.Request) {
		clientID := chi.URLParam(r, "clientID")
		if err := handler.DeleteClient(r.Context(), DeleteClientRequest{
			ClientID:                clientID,
			RegistrationAccessToken: registrationAccessTokenFromRequest(r, ""),
		}); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusNoContent, nil)
	})
}
