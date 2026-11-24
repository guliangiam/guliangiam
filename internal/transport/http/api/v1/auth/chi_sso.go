package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func registerSsoRoutes(router chi.Router, handler Handler) {
	// SSO Logout (front-channel)
	ssoLogoutHandler := func(w http.ResponseWriter, r *http.Request) {
		sessionID := ""
		if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
			sessionID = cookie.Value
		}

		result, err := handler.SsoLogout(r.Context(), sessionID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		ClearSessionCookie(w)
		route.WriteJSON(w, http.StatusOK, result)
	}
	router.Get("/sso-logout", ssoLogoutHandler)
	router.Post("/sso-logout", ssoLogoutHandler)
}
