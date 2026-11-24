package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func registerSharedRoutes(router chi.Router, handler Handler) {
	router.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		var request LoginRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := request.Validate(); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, sessionID, err := handler.Login(r.Context(), request.toCommand(r))
		if err != nil {
			mw.RecordLoginError(request.OrganizationSlug, "password", "login_failed")
			route.WriteError(w, r, err)
			return
		}

		mw.RecordLoginAttempt(request.OrganizationSlug, "password", "success")

		if sessionID != "" {
			SetOpaqueSessionCookie(w, sessionID)
		}
		route.WriteJSON(w, http.StatusOK, response)
	})
	router.Post("/native-sso-complete", func(w http.ResponseWriter, r *http.Request) {
		var request NativeSsoCompleteRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.CompleteNativeSso(r.Context(), request.AccessToken, request.ApplicationID); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("completed", "operation completed"))
	})
	router.Post("/logout", func(w http.ResponseWriter, r *http.Request) {
		// Try session cookie first. Logout treats absent/expired sessions as
		// idempotent success internally; any error it returns is a dependency
		// failure — the session may still be live server-side, so report it and
		// keep the cookie instead of claiming "logged_out" (BE-06).
		if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
			if err := handler.Logout(r.Context(), appauth.LogoutCommand{
				SessionID: cookie.Value,
				IssuerURL: platformconfig.IssuerURL(r),
			}); err != nil {
				route.WriteError(w, r, err)
				return
			}
		}

		// Clear session cookie
		ClearSessionCookie(w)

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("logged_out", "session ended"))
	})
	router.Patch("/account", func(w http.ResponseWriter, r *http.Request) {
		var request UpdateAccountRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.UpdateAccount(r.Context(), request.toCommand(requestFromHTTPRequest(r)))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})
	router.Get("/login-challenges/{challengeId}", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.GetLoginChallenge(r.Context(), loginChallengeCommandFromRequest(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})
	router.Post("/login-challenges/{challengeId}/approve", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.ApproveLoginChallenge(r.Context(), loginChallengeCommandFromRequest(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})
	router.Post("/login-challenges/{challengeId}/reject", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.RejectLoginChallenge(r.Context(), loginChallengeCommandFromRequest(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})
	router.Post("/exit-impersonate", func(w http.ResponseWriter, r *http.Request) {
		sessionCtx, ok := SessionContextFromContext(r.Context())
		if !ok {
			route.WriteError(w, r, apperrors.NewUnauthorized("unauthorized"))
			return
		}

		if sessionCtx.Username == "" {
			route.WriteError(w, r, apperrors.NewUnauthorized("cannot determine impersonated user"))
			return
		}

		// Resolve user by username and organization
		account, err := handler.GetAccount(r.Context(), GetAccountRequest{
			OrganizationSlug: sessionCtx.OrganizationSlug,
			Username:         sessionCtx.Username,
			AccessToken:      sessionCtx.AccessToken,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		// Account.Owner 是组织 slug，Account.Name 才是用户名；
		// ExitImpersonate 按 username 查用户，传错会 404。
		if err := handler.ExitImpersonate(r.Context(), sessionCtx.OrganizationSlug, account.Account.Name); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "impersonation session ended"))
	})
}

// loginChallengeCommandFromRequest binds the path parameter and the
// authenticated session principal. All login-challenge operations
// (read/approve/reject) are session-required; the application layer then
// verifies that the challenge belongs to that session principal.
func loginChallengeCommandFromRequest(r *http.Request) appauth.LoginChallengeCommand {
	command := appauth.LoginChallengeCommand{ChallengeID: chi.URLParam(r, "challengeId")}
	if session, ok := SessionContextFromContext(r.Context()); ok {
		command.SessionUserID = session.UserID
		command.SessionOrganizationID = session.OrganizationID
	}
	return command
}

// registerSessionReadRoutes mounts the read-only session-context endpoints the
// console calls on every bootstrap (masked contact for the login page, account
// and permission snapshot after login).
//
// 这些端点从认证面拆出来单独挂载：认证面整体使用 AuthRateLimit（10/min、burst 5）
// 是防撞库强度所需，但控制台登录后会连续读取 account + permissions + 其他引导数据，
// 用同一桶会在登录成功后立刻拿到 429。读端点改为 DefaultAPIRateLimit（registered
// policy `global-ip`），凭据类写端点仍留在严格桶内。
func registerSessionReadRoutes(router chi.Router, handler Handler) {
	router.Get("/contact", func(w http.ResponseWriter, r *http.Request) {
		orgSlug := r.URL.Query().Get("organizationSlug")
		username := r.URL.Query().Get("username")
		result, err := handler.GetMaskedContact(r.Context(), orgSlug, username)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Get("/account", func(w http.ResponseWriter, r *http.Request) {
		request := requestFromHTTPRequest(r)
		response, err := handler.GetAccount(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})
	router.Get("/permissions", func(w http.ResponseWriter, r *http.Request) {
		// Read session from SessionContextMiddleware-injected context (same as /auth/account).
		// sessionContextFromHeaders only accepts JWT Bearer tokens, but the browser
		// client authenticates via session cookie — hence the prior 401.
		sessionCtx, ok := SessionContextFromContext(r.Context())
		if !ok {
			route.WriteError(w, r, apperrors.NewUnauthorized("unauthorized"))
			return
		}

		perms, err := handler.GetCurrentUserPermissions(r.Context(), sessionCtx.Username, sessionCtx.OrganizationSlug)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, perms)
	})
}
