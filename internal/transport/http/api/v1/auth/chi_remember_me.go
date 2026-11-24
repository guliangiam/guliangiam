package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

const (
	rememberMeCookieName = "guliangiam_remember_me"
	// rememberMeCookiePath 把 Cookie 限定在认证端点，避免随业务请求外泄。
	rememberMeCookiePath   = "/api/v1/auth"
	rememberMeCookieMaxAge = int(appauth.RememberMeTTL / time.Second)
)

func registerRememberMeRoutes(router chi.Router, handler Handler) {
	router.Post("/remember-me/issue", func(w http.ResponseWriter, r *http.Request) {
		session, ok := rememberMeSession(w, r)
		if !ok {
			return
		}

		var request RememberMeIssueRequest
		if r.ContentLength > 0 {
			if err := route.DecodeJSON(r, &request); err != nil {
				route.WriteError(w, r, err)
				return
			}
		}

		result, err := handler.IssueRememberMe(r.Context(), session.OrganizationID, session.UserID, trimRememberMeToken(request.ApplicationID))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		setRememberMeCookie(w, result.Token)
		route.WriteJSON(w, http.StatusOK, result)
	})

	router.Post("/remember-me/exchange", func(w http.ResponseWriter, r *http.Request) {
		var request RememberMeExchangeRequest
		if r.ContentLength > 0 {
			if err := route.DecodeJSON(r, &request); err != nil {
				route.WriteError(w, r, err)
				return
			}
		}

		plaintext := trimRememberMeToken(request.Token)
		if plaintext == "" {
			plaintext = rememberMeTokenFromCookie(r)
		}

		result, err := handler.ExchangeRememberMe(r.Context(), plaintext)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		setRememberMeCookie(w, result.Token)
		route.WriteJSON(w, http.StatusOK, result)
	})

	router.Post("/remember-me/revoke", func(w http.ResponseWriter, r *http.Request) {
		session, ok := rememberMeSession(w, r)
		if !ok {
			return
		}

		var request RememberMeRevokeRequest
		if r.ContentLength > 0 {
			if err := route.DecodeJSON(r, &request); err != nil {
				route.WriteError(w, r, err)
				return
			}
		}

		if err := handler.RevokeRememberMe(r.Context(), session.OrganizationID, session.UserID, trimRememberMeToken(request.FamilyID)); err != nil {
			route.WriteError(w, r, err)
			return
		}

		clearRememberMeCookie(w)
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("revoked", "remember-me revoked"))
	})
}

func (h Handler) IssueRememberMe(ctx context.Context, organizationID, userID, applicationID string) (*appauth.RememberMeIssueResult, error) {
	return h.Service.IssueRememberMe(ctx, organizationID, userID, applicationID)
}

func (h Handler) ExchangeRememberMe(ctx context.Context, plaintext string) (*appauth.RememberMeExchangeResult, error) {
	return h.Service.ExchangeRememberMe(ctx, plaintext)
}

func (h Handler) RevokeRememberMe(ctx context.Context, organizationID, userID, familyID string) error {
	return h.Service.RevokeRememberMe(ctx, organizationID, userID, familyID)
}

// rememberMeSession mirrors trustedDeviceSession: org/user come from the
// verified session, never from client-supplied query/body/header.
func rememberMeSession(w http.ResponseWriter, r *http.Request) (SessionContext, bool) {
	session, ok := SessionContextFromContext(r.Context())
	if !ok || strings.TrimSpace(session.UserID) == "" || strings.TrimSpace(session.OrganizationID) == "" {
		route.WriteError(w, r, apperrors.NewUnauthorized("authentication required"))
		return SessionContext{}, false
	}

	organizationID := TrustedOrganizationID(r, "")
	if organizationID == "" || organizationID != session.OrganizationID {
		organizationID = strings.TrimSpace(session.OrganizationID)
	}
	session.OrganizationID = organizationID
	return session, true
}

func setRememberMeCookie(w http.ResponseWriter, token string) {
	if strings.TrimSpace(token) == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     rememberMeCookieName,
		Value:    token,
		Path:     rememberMeCookiePath,
		MaxAge:   rememberMeCookieMaxAge,
		HttpOnly: true,
		Secure:   sessionCookieSecure(),
		SameSite: http.SameSiteStrictMode,
	})
}

func clearRememberMeCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     rememberMeCookieName,
		Value:    "",
		Path:     rememberMeCookiePath,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   sessionCookieSecure(),
		SameSite: http.SameSiteStrictMode,
	})
}

func rememberMeTokenFromCookie(r *http.Request) string {
	cookie, err := r.Cookie(rememberMeCookieName)
	if err != nil || cookie == nil {
		return ""
	}
	return trimRememberMeToken(cookie.Value)
}
