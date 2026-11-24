package auth

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func registerStepUpRoutes(router chi.Router, handler Handler) {
	router.Post("/step-up/begin", func(w http.ResponseWriter, r *http.Request) {
		session, ok := stepUpSession(w, r)
		if !ok {
			return
		}

		var request StepUpBeginRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		result, err := handler.Service.BeginStepUp(r.Context(), request.toCommand(session))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, StepUpBeginResponse{
			ChallengeID:    result.ChallengeID,
			OperationScope: result.OperationScope,
			ExpiresAt:      result.ExpiresAt,
		})
	})

	router.Post("/step-up/finish", func(w http.ResponseWriter, r *http.Request) {
		session, ok := stepUpSession(w, r)
		if !ok {
			return
		}

		var request StepUpFinishRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		result, err := handler.Service.FinishStepUp(r.Context(), request.toCommand(session))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, StepUpFinishResponse{
			AssuranceLevel: result.AssuranceLevel,
			SessionID:      result.SessionID,
			OperationScope: result.OperationScope,
			ElevatedAt:     result.ElevatedAt,
		})
	})
}

// stepUpSession resolves the principal from the verified session only. All
// three identifiers must be present: an elevation is meaningful only when the
// server already knows which session and user it is raising.
func stepUpSession(w http.ResponseWriter, r *http.Request) (SessionContext, bool) {
	session, ok := SessionContextFromContext(r.Context())
	if !ok ||
		strings.TrimSpace(session.UserID) == "" ||
		strings.TrimSpace(session.OrganizationID) == "" ||
		strings.TrimSpace(session.ID) == "" {
		route.WriteError(w, r, apperrors.NewUnauthorized("authentication required"))
		return SessionContext{}, false
	}
	return session, true
}
