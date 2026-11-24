package dcr

import (
	"net/http"
	"strings"
)

// registrationAccessTokenFromRequest extracts the RFC 7591 §3.2.1 management
// credential.
//
// RFC 7591 §2.3 prescribes `Authorization: Bearer <registration_access_token>`.
// The request body is also accepted because the same value is returned to the
// client in the registration response and some clients echo it back in the body
// rather than in a header; both shapes resolve to one value, and the service
// layer compares it in constant time either way.
func registrationAccessTokenFromRequest(r *http.Request, bodyToken string) string {
	if header := strings.TrimSpace(r.Header.Get("Authorization")); header != "" {
		const prefix = "Bearer "
		if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
			if token := strings.TrimSpace(header[len(prefix):]); token != "" {
				return token
			}
		}
	}
	return strings.TrimSpace(bodyToken)
}
