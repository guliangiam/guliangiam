package users

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func registerRelationsRoutes(router chi.Router, handler Handler) {
	router.Delete("/api/v1/users/{id}/groups/{groupId}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.DeleteGroupMembership(r.Context(), organizationID, chi.URLParam(r, "id"), chi.URLParam(r, "groupId")); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})

	router.Delete("/api/v1/users/{id}/identities/{identityId}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.DeleteIdentity(r.Context(), organizationID, chi.URLParam(r, "id"), chi.URLParam(r, "identityId")); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})

	router.Delete("/api/v1/users/{id}/role-bindings/{roleId}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.DeleteRoleBinding(r.Context(), organizationID, chi.URLParam(r, "id"), chi.URLParam(r, "roleId")); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})
	router.Post("/api/v1/users/{id}/keys", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.RotateAccessKey(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/users/{id}/unlink", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request UnlinkIdentityRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.UnlinkIdentity(r.Context(), organizationID, chi.URLParam(r, "id"), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("unlinked", "resource unlinked"))
	})

	router.Post("/api/v1/users/{id}/impersonate", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		callerUsername := ""
		callerOrgID := ""
		var callerUserID string
		var callerOrgUUID uuid.UUID
		if session, ok := auth.SessionContextFromContext(r.Context()); ok {
			callerUsername = session.Username
			callerOrgID = session.OrganizationSlug
			callerUserID = session.UserID
			callerOrgUUID, _ = uuid.Parse(session.OrganizationID)
		}

		// Phase C-Step 3: impersonate 纳入 ScopeAuthorizer（RiskCritical）
		// 不管是否跨租户，impersonate 都是最高风险操作 → 需要 approved ticket。
		// ScopeAuthorizer Gate 2 会拦截所有 RiskCritical 请求检查 ticket。
		// Gate 4 compares SourceTenant with the real target tenant; the trusted
		// organization is the only source, so a forged query value cannot widen
		// the target for a non-platform-admin caller.
		if handler.ScopeAuthorizer != nil && callerUserID != "" {
			actorUUID, _ := uuid.Parse(callerUserID)
			req := scopeauthorizer.AuthorizationRequest{
				ActorID:      actorUUID,
				ActorType:    scopeauthorizer.PrincipalType(principalTypeFromSession(r.Context())),
				SourceTenant: callerOrgUUID,
				TargetTenant: targetTenantUUID(organizationID, callerOrgUUID),
				ResourceType: "users",
				Action:       "impersonate",
				RiskLevel:    scopeauthorizer.RiskCritical,
			}
			result, err := handler.ScopeAuthorizer.Authorize(r.Context(), req)
			if err != nil {
				route.WriteError(w, r, apperrors.NewForbidden("scope authorization failed for impersonate"))
				return
			}
			if !result.Allowed {
				route.WriteError(w, r, apperrors.NewForbidden(result.Reason))
				return
			}
		}

		response, err := handler.Impersonate(r.Context(), callerUsername, callerOrgID, organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/users/{id}/exit-impersonate", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		// {id} must be the identity the caller is currently operating as.
		// Without this check an authenticated caller could end (and thereby
		// revoke tokens for) any user id it can name.
		session, ok := auth.SessionContextFromContext(r.Context())
		if !ok || strings.TrimSpace(session.UserID) == "" {
			route.WriteError(w, r, apperrors.NewUnauthorized("cannot determine impersonated user"))
			return
		}
		if session.UserID != chi.URLParam(r, "id") {
			route.WriteError(w, r, apperrors.NewForbidden("cannot exit impersonation for another user"))
			return
		}

		if err := handler.ExitImpersonate(r.Context(), organizationID, chi.URLParam(r, "id")); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "impersonation session ended"))
	})

	router.Post("/api/v1/users/{id}/verify-identification", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request VerifyIdentificationRequest
		if r.ContentLength > 0 {
			if err := route.DecodeJSON(r, &request); err != nil {
				route.WriteError(w, r, err)
				return
			}
		}
		ticket, err := handler.VerifyIdentification(r.Context(), organizationID, chi.URLParam(r, "id"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if ticket.AlreadyVerified {
			route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("verified", "resource verified"))
			return
		}

		data := map[string]any{
			"verificationId": ticket.VerificationID,
			"provider":       ticket.Provider,
		}
		if ticket.RedirectURL != "" {
			data["redirectUrl"] = ticket.RedirectURL
		}
		if ticket.ExpiresAt != "" {
			data["expiresAt"] = ticket.ExpiresAt
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponseWithData("pending", "identity verification session created", data))
	})
	router.Post("/api/v1/users/{id}/set-password", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request SetPasswordRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.SetPassword(r.Context(), organizationID, chi.URLParam(r, "id"), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
	})
}

// targetTenantUUID prefers the trusted organization resolved for this request.
// A non-UUID value keeps the caller tenant so the scope authorizer still sees a
// concrete target; the application layer re-checks ownership with the same
// trusted organization.
func targetTenantUUID(organizationID string, fallback uuid.UUID) uuid.UUID {
	if parsed, err := uuid.Parse(organizationID); err == nil {
		return parsed
	}
	return fallback
}

// principalTypeFromSession 从 SessionContext 提取 PrincipalType。
// 空字符串 → 默认 "user"（ScopeAuthorizer Gate 3 会 deny 非 platform_admin）。
func principalTypeFromSession(ctx context.Context) string {
	if session, ok := auth.SessionContextFromContext(ctx); ok && session.PrincipalType != "" {
		return session.PrincipalType
	}
	return "user"
}
