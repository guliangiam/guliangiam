package mtls

import (
	"crypto/subtle"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ValidateCertificateBinding checks that a token's RFC 8705 confirmation value
// matches the certificate presented on the current request. It is the
// resource-server-side counterpart to BindTokenToClientCertificate.
//
// Semantics:
//   - token without cnf.x5t#S256: not certificate-bound -> nil (no constraint)
//   - token bound but request has no certificate: rejected
//   - thumbprints differ: rejected (constant-time comparison)
func ValidateCertificateBinding(metadata map[string]any, presentedThumbprint string) error {
	expected := ClientCertificateThumbprintFromMetadata(metadata)
	if expected == "" {
		return nil
	}
	presented := strings.TrimSpace(presentedThumbprint)
	if presented == "" {
		return apperrors.NewUnauthorized("token is certificate-bound but the request has no client certificate")
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(presented)) != 1 {
		return apperrors.NewUnauthorized("client certificate does not match the token confirmation")
	}
	return nil
}
