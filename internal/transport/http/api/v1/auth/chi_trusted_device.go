package auth

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func registerTrustedDeviceRoutes(router chi.Router, handler Handler) {
	router.Post("/trusted-devices/begin", func(w http.ResponseWriter, r *http.Request) {
		session, ok := trustedDeviceSession(w, r)
		if !ok {
			return
		}

		result, err := handler.BeginTrustedDeviceRegistration(r.Context(), appauth.TrustedDeviceBeginCommand{
			OrganizationID: session.OrganizationID,
			UserID:         session.UserID,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})

	router.Post("/trusted-devices", func(w http.ResponseWriter, r *http.Request) {
		session, ok := trustedDeviceSession(w, r)
		if !ok {
			return
		}

		var request TrustedDeviceRegisterRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		result, err := handler.RegisterTrustedDevice(r.Context(), request.toCommand(session, clientIPFromRequest(r), strings.TrimSpace(r.UserAgent())))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		// 明文凭据只在注册响应下发一次（HttpOnly Cookie）；服务端不落明文，
		// 响应体的 deviceCredentialId 仍是掩码。必须在写响应体之前设置。
		SetTrustedDeviceCredentialCookie(w, request.DeviceCredentialID)

		route.WriteJSON(w, http.StatusOK, result)
	})

	router.Get("/trusted-devices", func(w http.ResponseWriter, r *http.Request) {
		session, ok := trustedDeviceSession(w, r)
		if !ok {
			return
		}

		result, err := handler.ListTrustedDevices(r.Context(), session.OrganizationID, session.UserID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})

	router.Delete("/trusted-devices/{deviceId}", func(w http.ResponseWriter, r *http.Request) {
		session, ok := trustedDeviceSession(w, r)
		if !ok {
			return
		}

		if err := handler.RevokeTrustedDevice(r.Context(), session.OrganizationID, session.UserID, chi.URLParam(r, "deviceId")); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("revoked", "resource revoked"))
	})
}

// trustedDeviceSession resolves the self-service principal from the verified
// session only. TrustedOrganizationID is consulted for the tenant scope but the
// session organization remains authoritative — a forged organizationId can
// never move a device write into another tenant.
func trustedDeviceSession(w http.ResponseWriter, r *http.Request) (SessionContext, bool) {
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
