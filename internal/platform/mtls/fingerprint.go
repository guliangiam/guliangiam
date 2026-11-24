// Package mtls provides RFC 8705 client-certificate helpers used by
// sender-constrained tokens (ADR-014). It intentionally contains no TLS
// termination policy: the HTTP server/gateway decides when to request client
// certificates, and this package derives the x5t#S256 binding from the
// in-process TLS state, or from an explicitly configured trusted-proxy header
// (see PeerCertificatesFromRequest).
package mtls

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

// ErrNoPeerCertificate means the request did not carry a TLS client
// certificate.
var ErrNoPeerCertificate = errors.New("no TLS peer certificate")

// LoadCertPoolFromFile loads a PEM CA bundle used to verify the certificate
// chain of RFC 8705 tls_client_auth clients. An empty path returns (nil, nil);
// callers must treat a nil pool as "no trusted CA configured" and fail closed.
func LoadCertPoolFromFile(path string) (*x509.CertPool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	pemData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemData) {
		return nil, errors.New("client CA bundle contains no valid certificates")
	}
	return pool, nil
}

// VerifyChainAgainstRoots verifies the peer certificate chain against the
// configured trusted roots. The leaf must come from the connection's TLS state;
// intermediaries are taken only from the same chain. A nil root pool means no
// trusted CA is configured and the caller must reject the request.
func VerifyChainAgainstRoots(peerCertificates []*x509.Certificate, roots *x509.CertPool, now time.Time) error {
	if roots == nil || len(peerCertificates) == 0 {
		return ErrNoPeerCertificate
	}
	intermediates := x509.NewCertPool()
	for _, cert := range peerCertificates[1:] {
		intermediates.AddCert(cert)
	}
	_, err := peerCertificates[0].Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	return err
}

// Thumbprint returns the RFC 8705 x5t#S256 confirmation value: the
// base64url-encoded SHA-256 digest of the DER-encoded certificate.
func Thumbprint(cert *x509.Certificate) (string, error) {
	if cert == nil || len(cert.Raw) == 0 {
		return "", ErrNoPeerCertificate
	}
	sum := sha256.Sum256(cert.Raw)
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// ThumbprintFromRequest derives the certificate thumbprint from the same chain
// PeerCertificatesFromRequest returns, so in-process TLS and the explicit
// trusted-proxy mode cannot disagree about which certificate authenticated the
// client. It returns ErrNoPeerCertificate when no chain is available.
func ThumbprintFromRequest(r *http.Request) (string, error) {
	chain := PeerCertificatesFromRequest(r)
	if len(chain) == 0 {
		return "", ErrNoPeerCertificate
	}
	return Thumbprint(chain[0])
}

// BindTokenToClientCertificate writes RFC 8705 confirmation metadata
// ("cnf" map with x5t#S256) and marks tokenBinding=mTLS. Existing cnf
// members (for example a DPoP jkt) are preserved so a token can carry both
// confirmations during migration.
func BindTokenToClientCertificate(metadata map[string]any, thumbprint string) map[string]any {
	if metadata == nil {
		metadata = map[string]any{}
	}
	thumbprint = strings.TrimSpace(thumbprint)
	if thumbprint == "" {
		return metadata
	}
	cnf, _ := metadata["cnf"].(map[string]any)
	if cnf == nil {
		cnf = map[string]any{}
	}
	cnf["x5t#S256"] = thumbprint
	metadata["cnf"] = cnf
	metadata["tokenBinding"] = "mTLS"
	return metadata
}

// ClientCertificateThumbprintFromMetadata reads cnf.x5t#S256 back from token
// metadata. It returns "" for unbound tokens.
func ClientCertificateThumbprintFromMetadata(metadata map[string]any) string {
	cnf, _ := metadata["cnf"].(map[string]any)
	if cnf == nil {
		return ""
	}
	thumbprint, _ := cnf["x5t#S256"].(string)
	return strings.TrimSpace(thumbprint)
}
