package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func registerSocialRoutes(router chi.Router, handler Handler) {
	router.Get("/app-login", func(w http.ResponseWriter, r *http.Request) {
		orgSlug := r.URL.Query().Get("organizationSlug")
		if orgSlug == "" {
			route.WriteError(w, r, apperrors.NewInvalidArgument("organizationSlug is required"))
			return
		}
		appSlug := r.URL.Query().Get("applicationSlug")

		result, err := handler.GetAppLogin(r.Context(), orgSlug, appSlug)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Get("/social/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		providerID := r.URL.Query().Get("providerId")

		if code == "" || providerID == "" {
			route.WriteError(w, r, apperrors.NewInvalidArgument("code and providerId are required"))
			return
		}

		command := appauth.SocialLoginCommand{
			ProviderID: providerID,
			Code:       code,
			State:      state,
			IPAddress:  r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		}

		result, sessionID, err := handler.LoginWithProvider(r.Context(), command)
		if err != nil {
			mw.RecordLoginError("", "social", "callback_failed")
			route.WriteError(w, r, err)
			return
		}

		mw.RecordLoginAttempt("", "social", "success")

		if sessionID != "" {
			SetOpaqueSessionCookie(w, sessionID)
		}

		// If the login result contains a redirect URL, redirect the browser.
		if result != nil && result.RedirectTo != "" {
			http.Redirect(w, r, result.RedirectTo, http.StatusFound)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Post("/social/login", func(w http.ResponseWriter, r *http.Request) {
		var request SocialLoginRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := request.Validate(); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, sessionID, err := handler.LoginWithProvider(r.Context(), request.toCommand(r))
		if err != nil {
			mw.RecordLoginError(request.OrganizationSlug, "social", "login_failed")
			route.WriteError(w, r, err)
			return
		}

		mw.RecordLoginAttempt(request.OrganizationSlug, "social", "success")

		if sessionID != "" {
			SetOpaqueSessionCookie(w, sessionID)
		}
		route.WriteJSON(w, http.StatusOK, response)
	})
	router.Post("/social/authorize", func(w http.ResponseWriter, r *http.Request) {
		var request SocialAuthorizeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		redirectURL, err := handler.GetSocialRedirectURL(r.Context(), request.toCommand())
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, map[string]any{
			"redirectUrl": redirectURL,
		})
	})
	router.Post("/login/ldap", func(w http.ResponseWriter, r *http.Request) {
		var request LdapLoginRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := request.Validate(); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, sessionID, err := handler.LoginWithLDAP(r.Context(), request.toCommand(r))
		if err != nil {
			mw.RecordLoginError(request.OrganizationSlug, "ldap", "login_failed")
			route.WriteError(w, r, err)
			return
		}

		mw.RecordLoginAttempt(request.OrganizationSlug, "ldap", "success")

		if sessionID != "" {
			SetOpaqueSessionCookie(w, sessionID)
		}
		route.WriteJSON(w, http.StatusOK, response)
	})
}
