package users

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func registerMFARoutes(router chi.Router, handler Handler) {
	registerTrustedDeviceRoutes(router, handler)

	router.Put("/api/v1/users/{id}/mfa-methods", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request ReplaceUserMfaMethodsRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.ReplaceMfaMethods(r.Context(), organizationID, chi.URLParam(r, "id"), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
	})
	router.Post("/api/v1/users/{id}/mfa-setup/initiate", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request InitiateUserMfaSetupRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.InitiateMfaSetup(r.Context(), organizationID, chi.URLParam(r, "id"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/users/{id}/mfa-setup/verify", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request VerifyUserMfaSetupRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.VerifyMfaSetup(r.Context(), organizationID, chi.URLParam(r, "id"), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("verified", "resource verified"))
	})

	router.Post("/api/v1/users/{id}/mfa-setup/enable", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request EnableUserMfaSetupRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		sessionID, err := handler.EnableMfaSetup(r.Context(), organizationID, chi.URLParam(r, "id"), auth.ExtractPreviousSessionIDFromRequest(r), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if sessionID != "" {
			auth.SetOpaqueSessionCookie(w, sessionID)
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("enabled", "resource enabled"))
	})
	// WebAuthn credentials sub-resource: list credentials for a user.
	router.Get("/api/v1/users/{id}/webauthn-credentials", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.ListWebAuthnCredentials(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// WebAuthn credentials sub-resource: delete a credential for a user.
	router.Delete("/api/v1/users/{id}/webauthn-credentials/{credentialId}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		sessionID, err := handler.DeleteWebAuthnCredential(r.Context(), organizationID, chi.URLParam(r, "id"), chi.URLParam(r, "credentialId"), auth.ExtractPreviousSessionIDFromRequest(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if sessionID != "" {
			auth.SetOpaqueSessionCookie(w, sessionID)
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})

	// 凭证状态迁移（§31.3.3）：吊销/停用/恢复。三个端点共用一段处理逻辑，只有目标状态不同；
	// 吊销与停用会级联撤销该用户全部会话与令牌，因此调用者会话可能被轮换（写回 Cookie）。
	updateCredentialStatus := func(status appusers.WebAuthnCredentialStatus) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
			if err != nil {
				route.WriteError(w, r, err)
				return
			}
			sessionID, err := handler.UpdateWebAuthnCredentialStatus(
				r.Context(), organizationID, chi.URLParam(r, "id"), chi.URLParam(r, "credentialId"),
				status, auth.ExtractPreviousSessionIDFromRequest(r))
			if err != nil {
				route.WriteError(w, r, err)
				return
			}
			if sessionID != "" {
				auth.SetOpaqueSessionCookie(w, sessionID)
			}

			route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse(string(status), "credential status updated"))
		}
	}

	router.Post("/api/v1/users/{id}/webauthn-credentials/{credentialId}/revoke",
		updateCredentialStatus(appusers.WebAuthnCredentialStatusRevoked))
	router.Post("/api/v1/users/{id}/webauthn-credentials/{credentialId}/deactivate",
		updateCredentialStatus(appusers.WebAuthnCredentialStatusDeactivated))
	router.Post("/api/v1/users/{id}/webauthn-credentials/{credentialId}/restore",
		updateCredentialStatus(appusers.WebAuthnCredentialStatusActive))

	// Single credential detail.
	router.Get("/api/v1/users/{id}/webauthn-credentials/{credentialId}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.GetWebAuthnCredential(r.Context(), organizationID, chi.URLParam(r, "id"), chi.URLParam(r, "credentialId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	// Rename a credential.
	router.Post("/api/v1/users/{id}/webauthn-credentials/{credentialId}/rename", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request RenameCredentialRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.RenameWebAuthnCredential(r.Context(), organizationID, chi.URLParam(r, "id"), chi.URLParam(r, "credentialId"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	// Server-driven registration ceremony: begin issues a one-time challenge.
	router.Post("/api/v1/users/{id}/webauthn-credentials/registration/begin", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request AdminRegistrationBeginRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.BeginWebAuthnCredentialRegistration(r.Context(), organizationID, chi.URLParam(r, "id"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	// Server-driven registration ceremony: finish derives credential material
	// from attestationObject and persists the credential.
	router.Post("/api/v1/users/{id}/webauthn-credentials/registration/finish", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request AdminRegistrationFinishRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.FinishWebAuthnCredentialRegistration(r.Context(), organizationID, chi.URLParam(r, "id"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})
}
