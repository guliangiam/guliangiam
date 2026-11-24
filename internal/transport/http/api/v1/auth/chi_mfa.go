package auth

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func registerMfaRoutes(router chi.Router, handler Handler) {
	router.Post("/mfa/push/initiate", func(w http.ResponseWriter, r *http.Request) {
		var request InitiatePushMfaRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		cmd := request.toCommand()
		userID, ok := bindPushMfaUserID(w, r, cmd.UserID)
		if !ok {
			return
		}
		cmd.UserID = userID

		result, err := handler.InitiatePushMfa(r.Context(), cmd)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Post("/mfa/push/verify", func(w http.ResponseWriter, r *http.Request) {
		var request VerifyPushMfaRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		cmd := request.toCommand()
		userID, ok := bindPushMfaUserID(w, r, "")
		if !ok {
			return
		}
		cmd.UserID = userID

		result, err := handler.VerifyPushMfa(r.Context(), cmd)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Post("/mfa/push/enable", func(w http.ResponseWriter, r *http.Request) {
		var request EnablePushMfaRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		cmd := request.toCommand()
		userID, ok := bindPushMfaUserID(w, r, cmd.UserID)
		if !ok {
			return
		}
		cmd.UserID = userID
		cmd.PreviousSessionID = ExtractPreviousSessionIDFromRequest(r)
		sessionID, err := handler.EnablePushMfa(r.Context(), cmd)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if sessionID != "" {
			SetOpaqueSessionCookie(w, sessionID)
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "operation succeeded"))
	})
	router.Post("/mfa/push/disable", func(w http.ResponseWriter, r *http.Request) {
		var request DisablePushMfaRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		cmd := request.toCommand()
		userID, ok := bindPushMfaUserID(w, r, cmd.UserID)
		if !ok {
			return
		}
		cmd.UserID = userID
		cmd.PreviousSessionID = ExtractPreviousSessionIDFromRequest(r)
		sessionID, err := handler.DisablePushMfa(r.Context(), cmd)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if sessionID != "" {
			SetOpaqueSessionCookie(w, sessionID)
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "operation succeeded"))
	})
}

// bindPushMfaUserID resolves the subject of every push MFA endpoint from the
// verified session, never from the request body. `userId` stays in the payload
// as a compatibility echo, but an explicit different value is rejected (403)
// instead of silently rewritten: callers must not believe they acted on
// another account. Verify passes an empty requestedUserID because its payload
// has no user field; the challenge owner is still checked in the service.
func bindPushMfaUserID(w http.ResponseWriter, r *http.Request, requestedUserID string) (string, bool) {
	session, ok := SessionContextFromContext(r.Context())
	if !ok || strings.TrimSpace(session.UserID) == "" {
		route.WriteError(w, r, apperrors.NewUnauthorized("authentication required"))
		return "", false
	}
	if requested := strings.TrimSpace(requestedUserID); requested != "" && requested != session.UserID {
		route.WriteError(w, r, apperrors.NewForbidden("cannot modify push mfa for another user"))
		return "", false
	}
	return session.UserID, true
}
