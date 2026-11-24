package users

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// registerAdminMFARoutes registers the platform-only user MFA administration
// endpoints. The route is not tenant-scoped: the target user's organization is
// taken from the user row inside the application service.
func registerAdminMFARoutes(router chi.Router, handler Handler) {
	router.Post("/api/v1/platform/users/{id}/mfa/totp/admin-rebind", func(w http.ResponseWriter, r *http.Request) {
		session, ok := authtransport.SessionContextFromContext(r.Context())
		if !ok || strings.TrimSpace(session.UserID) == "" {
			route.WriteError(w, r, apperrors.NewUnauthorized("authentication required"))
			return
		}

		principalType := principalTypeFromSession(r.Context())
		if principalType != string(scopeauthorizer.PrincipalPlatformAdmin) {
			route.WriteError(w, r, apperrors.NewForbidden("platform administrator required"))
			return
		}

		var request AdminRebindTOTPRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		command := request.toCommand(session.UserID, principalType, chi.URLParam(r, "id"))
		if err := handler.AdminRebindTOTP(r.Context(), command); err != nil {
			route.WriteError(w, r, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// AdminRebindTOTP forwards the platform-only TOTP reset to the application
// service. The transport never supplies an organization id from the request.
func (h Handler) AdminRebindTOTP(ctx context.Context, command appusers.AdminRebindTOTPCommand) error {
	return h.Service.AdminRebindTOTP(ctx, command)
}
