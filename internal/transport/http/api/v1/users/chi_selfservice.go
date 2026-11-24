package users

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// registerSelfServiceRoutes mounts the session-bound self-service endpoints under
// /api/v1/auth/* that let an ordinary member manage their own MFA factors, unlink
// a third-party identity, read their own application consents, read/merge their
// own profile, start real-name verification and enroll/revoke their own FaceID —
// without any users:read/write/delete management permission.
//
// Security model (fail-close, 架构 §8.8):
//   - The caller identity is taken ONLY from the trusted session context injected
//     by SessionContextMiddleware (session.UserID / session.OrganizationID). The
//     request body/query never names the target user or tenant; the reused users
//     Handler methods resolve ownership through GetByIDForOrganization, so a
//     forged userID/orgID cannot move the operation onto another principal.
//   - APIAuthorizationMiddleware skips Casbin for these paths via
//     auth.isAuthRequiringSession but still requires a session (no session → 401).
//     The handler re-asserts the session here so the route is safe even when
//     mounted directly on a router without that middleware (unit tests).
//   - The group stamps the authentication surface so metrics/audit/OpenAPI tags
//     agree with the route declarations (SurfaceAuthentication), even though the
//     users module is otherwise mounted on the management face.
func registerSelfServiceRoutes(router chi.Router, handler Handler) {
	router.Group(func(r chi.Router) {
		r.Use(route.WithSurface(route.SurfaceAuthentication))

		r.Post("/api/v1/auth/mfa-setup/initiate", func(w http.ResponseWriter, req *http.Request) {
			orgID, userID, ok := selfServicePrincipal(w, req)
			if !ok {
				return
			}
			var request InitiateUserMfaSetupRequest
			if err := route.DecodeJSON(req, &request); err != nil {
				route.WriteError(w, req, err)
				return
			}
			response, err := handler.InitiateMfaSetup(req.Context(), orgID, userID, request)
			if err != nil {
				route.WriteError(w, req, err)
				return
			}
			route.WriteJSON(w, http.StatusOK, response)
		})

		r.Post("/api/v1/auth/mfa-setup/verify", func(w http.ResponseWriter, req *http.Request) {
			orgID, userID, ok := selfServicePrincipal(w, req)
			if !ok {
				return
			}
			var request VerifyUserMfaSetupRequest
			if err := route.DecodeJSON(req, &request); err != nil {
				route.WriteError(w, req, err)
				return
			}
			if err := handler.VerifyMfaSetup(req.Context(), orgID, userID, request); err != nil {
				route.WriteError(w, req, err)
				return
			}
			route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("verified", "resource verified"))
		})

		r.Post("/api/v1/auth/mfa-setup/enable", func(w http.ResponseWriter, req *http.Request) {
			orgID, userID, ok := selfServicePrincipal(w, req)
			if !ok {
				return
			}
			var request EnableUserMfaSetupRequest
			if err := route.DecodeJSON(req, &request); err != nil {
				route.WriteError(w, req, err)
				return
			}
			sessionID, err := handler.EnableMfaSetup(req.Context(), orgID, userID, auth.ExtractPreviousSessionIDFromRequest(req), request)
			if err != nil {
				route.WriteError(w, req, err)
				return
			}
			if sessionID != "" {
				auth.SetOpaqueSessionCookie(w, sessionID)
			}
			route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("enabled", "resource enabled"))
		})

		r.Put("/api/v1/auth/mfa-methods", func(w http.ResponseWriter, req *http.Request) {
			orgID, userID, ok := selfServicePrincipal(w, req)
			if !ok {
				return
			}
			var request ReplaceUserMfaMethodsRequest
			if err := route.DecodeJSON(req, &request); err != nil {
				route.WriteError(w, req, err)
				return
			}
			if err := handler.ReplaceMfaMethods(req.Context(), orgID, userID, request); err != nil {
				route.WriteError(w, req, err)
				return
			}
			route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
		})

		r.Put("/api/v1/auth/preferred-mfa", func(w http.ResponseWriter, req *http.Request) {
			orgID, userID, ok := selfServicePrincipal(w, req)
			if !ok {
				return
			}
			var request UpdateUserPreferredMfaRequest
			if err := route.DecodeJSON(req, &request); err != nil {
				route.WriteError(w, req, err)
				return
			}
			if err := handler.UpdatePreferredMfa(req.Context(), orgID, userID, request); err != nil {
				route.WriteError(w, req, err)
				return
			}
			route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
		})

		r.Delete("/api/v1/auth/identities/{identityId}", func(w http.ResponseWriter, req *http.Request) {
			orgID, userID, ok := selfServicePrincipal(w, req)
			if !ok {
				return
			}
			if err := handler.DeleteIdentity(req.Context(), orgID, userID, chi.URLParam(req, "identityId")); err != nil {
				route.WriteError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		r.Get("/api/v1/auth/consents", func(w http.ResponseWriter, req *http.Request) {
			orgID, userID, ok := selfServicePrincipal(w, req)
			if !ok {
				return
			}
			response, err := handler.ListUserConsents(req.Context(), orgID, userID)
			if err != nil {
				route.WriteError(w, req, err)
				return
			}
			route.WriteJSON(w, http.StatusOK, response)
		})

		// Self profile read: same UserDetail shape as the admin GET /users/{id}
		// so the account page can reuse userDetailSchema. no-store keeps the
		// PII-bearing projection out of shared HTTP caches.
		r.Get("/api/v1/auth/profile", func(w http.ResponseWriter, req *http.Request) {
			orgID, userID, ok := selfServicePrincipal(w, req)
			if !ok {
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			response, err := handler.Get(req.Context(), GetUserRequest{ID: userID, OrganizationID: orgID})
			if err != nil {
				route.WriteError(w, req, err)
				return
			}
			route.WriteJSON(w, http.StatusOK, response)
		})

		// Self profile partial merge: the strict decoder rejects any payload
		// that names email/phone (contact binding lives on /auth/account) or a
		// userID/organizationId; only the supplied fields are overlaid.
		r.Patch("/api/v1/auth/profile", func(w http.ResponseWriter, req *http.Request) {
			orgID, userID, ok := selfServicePrincipal(w, req)
			if !ok {
				return
			}
			var request UpdateSelfProfileRequest
			if err := route.DecodeJSON(req, &request); err != nil {
				route.WriteError(w, req, err)
				return
			}
			if err := handler.UpdateSelfProfile(req.Context(), orgID, userID, request); err != nil {
				route.WriteError(w, req, err)
				return
			}
			route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
		})

		// Self real-name verification: mirrors the admin route's ticket shape.
		// Fail-close — VerifyIdentification only returns a provider ticket; it
		// never issues a token/session/cookie, so no login state is introduced.
		r.Post("/api/v1/auth/verify-identification", func(w http.ResponseWriter, req *http.Request) {
			orgID, userID, ok := selfServicePrincipal(w, req)
			if !ok {
				return
			}
			var request VerifyIdentificationRequest
			if req.ContentLength > 0 {
				if err := route.DecodeJSON(req, &request); err != nil {
					route.WriteError(w, req, err)
					return
				}
			}
			ticket, err := handler.VerifyIdentification(req.Context(), orgID, userID, request)
			if err != nil {
				route.WriteError(w, req, err)
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

		// Self FaceID enrollment. Plural "faceids" is deliberate: the public
		// login handshake owns singular /api/v1/auth/faceid and stays untouched.
		// The multipart contract is shared with the admin route.
		r.Post("/api/v1/auth/faceids", func(w http.ResponseWriter, req *http.Request) {
			orgID, userID, ok := selfServicePrincipal(w, req)
			if !ok {
				return
			}
			mediaType, data, err := parseFaceImageUpload(req)
			if err != nil {
				route.WriteError(w, req, err)
				return
			}
			faceID, err := handler.EnrollFaceID(
				req.Context(),
				orgID,
				userID,
				req.FormValue("providerId"),
				req.FormValue("deviceInfo"),
				mediaType,
				data,
			)
			if err != nil {
				route.WriteError(w, req, err)
				return
			}
			route.WriteJSON(w, http.StatusCreated, faceID)
		})

		// Self FaceID revocation: blocks face-id login and clears the legacy
		// marker in one audited transaction (app returns 404 when none active).
		r.Delete("/api/v1/auth/faceids", func(w http.ResponseWriter, req *http.Request) {
			orgID, userID, ok := selfServicePrincipal(w, req)
			if !ok {
				return
			}
			if err := handler.RevokeFaceID(req.Context(), orgID, userID); err != nil {
				route.WriteError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})
}

// selfServicePrincipal extracts the caller identity from the trusted session and
// fail-closes with 401 when there is no session or the session carries no user id.
// The organization id is returned verbatim from the session (mirroring the
// webauthn self-service routes); the application layer's requireOrganizationID
// rejects an empty tenant before any repository work.
func selfServicePrincipal(w http.ResponseWriter, r *http.Request) (organizationID, userID string, ok bool) {
	session, found := auth.SessionContextFromContext(r.Context())
	if !found {
		route.WriteError(w, r, apperrors.NewUnauthorized("a session is required"))
		return "", "", false
	}
	userID = strings.TrimSpace(session.UserID)
	if userID == "" {
		route.WriteError(w, r, apperrors.NewUnauthorized("a session is required"))
		return "", "", false
	}
	return session.OrganizationID, userID, true
}
