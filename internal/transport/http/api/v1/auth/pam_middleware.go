package auth

import (
	"context"
	"net/http"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

type PrivilegedOperationAuditor interface {
	AuditPrivilegedOperation(ctx context.Context, orgID, sessionID, actorID, action, resource string) error
}

// PrivilegedAuditMiddleware records every authorized operation performed by a
// repository-backed privileged session. The auditor also rechecks the live JIT
// window, so a stale request context cannot bypass expiration or revocation.
func PrivilegedAuditMiddleware(auditor PrivilegedOperationAuditor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := SessionContextFromContext(r.Context())
			if !ok || !session.IsPrivileged {
				next.ServeHTTP(w, r)
				return
			}
			if auditor == nil {
				route.WriteError(w, r, apperrors.NewUnavailable("privileged audit is not configured"))
				return
			}
			if err := auditor.AuditPrivilegedOperation(r.Context(), session.OrganizationID, session.ID, session.UserID, r.Method, r.URL.Path); err != nil {
				route.WriteError(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
