package messages

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	appmessages "github.com/guliangiam/guliangiam/internal/app/messages"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Post("/api/v1/messages/email", func(w http.ResponseWriter, r *http.Request) {
		var request SendEmailRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.SendEmail(r.Context(), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "email sent"))
	})

	router.Post("/api/v1/messages/sms", func(w http.ResponseWriter, r *http.Request) {
		var request SendSmsRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.SendSms(r.Context(), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "sms sent"))
	})

	router.Post("/api/v1/messages/notification", func(w http.ResponseWriter, r *http.Request) {
		var request SendNotificationRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		// The body organizationId is a compatibility echo only. The authenticated
		// tenant scope decides where the notification is delivered, so a forged
		// value cannot move the send into another tenant. The single exception is
		// an explicit cross-tenant target from a platform administrator, which
		// TenantFilterMiddleware escalated to super_admin and
		// TrustedOrganizationID honours by design.
		trustedOrgID := authtransport.TrustedOrganizationID(r, request.OrganizationID)
		if trustedOrgID == "" {
			route.WriteError(w, r, apperrors.NewInvalidArgument("organizationId is required"))
			return
		}
		request.OrganizationID = trustedOrgID

		if err := handler.SendNotification(r.Context(), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "notification sent"))
	})
}

// NewHandler creates a new messages handler.
func NewHandler(svc appmessages.Service) Handler {
	return Handler{Service: svc}
}
