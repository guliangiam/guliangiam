package verifications

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/middleware"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/verifications", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request := ListVerificationsRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: organizationID,
			Type:           r.URL.Query().Get("type"),
			Status:         r.URL.Query().Get("status"),
			Recipient:      r.URL.Query().Get("recipient"),
		}

		response, err := handler.List(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/verifications/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.Get(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// Send is a delivery endpoint: stack the registered auth IP budget with a
	// recipient-keyed bucket so one source cannot grind many recipients and
	// one recipient cannot be SMS/mail-bombed through rotating IPs (BE-A-04).
	// Both use the existing platform middleware (in-process or Redis-backed).
	router.With(
		middleware.RateLimit(middleware.AuthRateLimit),
		middleware.RateLimit(middleware.AuthRateLimit, middleware.WithKeyFunc(sendRecipientRateLimitKey)),
	).Post("/api/v1/auth/verification-code/send", func(w http.ResponseWriter, r *http.Request) {
		var request SendVerificationCodeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		// Trusted client IP only (BE-A-03): the captcha gate and rate budget
		// must not be escapable by rotating a caller-supplied XFF header.
		request.ClientIP = util.ClientIP(r)

		response, err := handler.SendVerificationCode(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/auth/verification-code/verify", func(w http.ResponseWriter, r *http.Request) {
		var request VerifyCodeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		// Trusted client IP only: X-Forwarded-For is honoured solely when the
		// socket peer is a configured trusted proxy (BE-A-03). A raw XFF read
		// let callers rotate the header to escape the per-IP attempt budget.
		request.ClientIP = util.ClientIP(r)

		if err := handler.VerifyCode(r.Context(), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("verified", "resource verified"))
	})
}

// trustedOrgID resolves the tenant a request may act on. A caller-supplied
// organizationId is honoured only for platform administrators; otherwise the
// authenticated tenant scope wins. An empty result is a client error: on
// dialects without RLS an empty organization_id degrades to "all tenants".
// The two /auth/verification-code endpoints are public: their body
// organizationId selects the tenant for the recipient, not an actor scope.
func trustedOrgID(r *http.Request, requested string) (string, error) {
	orgID := authtransport.TrustedOrganizationID(r, requested)
	if orgID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	return orgID, nil
}
