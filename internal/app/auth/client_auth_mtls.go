package auth

import (
	"context"
	"crypto/x509"
	"log/slog"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	platformmtls "github.com/guliangiam/guliangiam/internal/platform/mtls"
)

// clientCertificateMatches authenticates a client that presents a TLS client
// certificate issued by the in-process TLS terminator (RFC 8705 §2).
//
// Trust boundary: only certificates from the Go TLS connection state are
// considered. The transport layer passes r.TLS.PeerCertificates verbatim and
// never reads any reverse-proxy client-certificate header — X-Client-Cert,
// X-Forwarded-Client-Cert and X-SSL-Client-* are ignored by design, because a
// client can forge them whenever TLS terminates upstream.
//
// The server uses tls.RequestClientCert (see internal/bootstrap/server.go), so
// the handshake does NOT reject an untrusted client certificate; it only makes
// the certificate optional. An untrusted certificate therefore reaches this
// function and must be rejected here: the four gates below are the only
// authentication decision, and each one fails closed. A certificate that the
// TLS stack merely accepted can never authenticate a client on its own.
//
// Behaviour per registered method:
//   - tls_client_auth: the chain must verify against the configured trusted CA
//     pool, then the leaf's x5t#S256 must match a registered row. No CA
//     configured means fail-closed.
//   - self_signed_tls_client_auth: no chain verification; the leaf's x5t#S256
//     must match a registered row byte-for-byte (row registered for the same
//     method).
//
// Any failure is reported as false so callers emit the same invalid_client
// error as an unknown client or a wrong secret.
func (s *Service) clientCertificateMatches(ctx context.Context, tx appuow.UnitOfWork, application *domain.Application, peerCertificates []*x509.Certificate) bool {
	authMethod := application.EffectiveTokenEndpointAuthMethod()
	switch authMethod {
	case domain.TokenEndpointAuthMethodTLSClientAuth, domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth:
	default:
		return false
	}
	if len(peerCertificates) == 0 || peerCertificates[0] == nil {
		return false
	}
	leaf := peerCertificates[0]
	thumbprint, err := platformmtls.Thumbprint(leaf)
	if err != nil {
		return false
	}
	now := s.Clock.Now()
	// Fingerprint prefix only: the certificate body must never reach the logs.
	prefix := thumbprint
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}

	if authMethod == domain.TokenEndpointAuthMethodTLSClientAuth {
		if err := platformmtls.VerifyChainAgainstRoots(peerCertificates, s.TLSClientCAs, now); err != nil {
			slog.DebugContext(ctx, "mTLS client certificate rejected: chain not trusted",
				"client_id", application.ClientID, "fingerprint_prefix", prefix)
			return false
		}
	}

	certificates, err := tx.Applications().ListActiveClientCertificates(ctx, application.OrganizationID, application.ID)
	if err != nil {
		slog.WarnContext(ctx, "mTLS client certificate lookup failed",
			"client_id", application.ClientID, "fingerprint_prefix", prefix)
		return false
	}
	for _, certificate := range certificates {
		if certificate == nil || !certificate.UsableAt(now) {
			continue
		}
		// The row must be registered for the same method: a self-signed row
		// cannot satisfy tls_client_auth and vice versa.
		if strings.TrimSpace(certificate.AuthMethod) != authMethod {
			continue
		}
		// Exact, case-sensitive comparison of the RFC 8705 x5t#S256 value:
		// no trimming, padding or alphabet normalisation.
		if certificate.FingerprintSHA256 != thumbprint {
			continue
		}
		return true
	}
	slog.DebugContext(ctx, "mTLS client certificate rejected: no matching registration",
		"client_id", application.ClientID, "fingerprint_prefix", prefix)
	return false
}
