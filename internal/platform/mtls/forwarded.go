package mtls

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/url"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/config"
)

const (
	// maxForwardedChainBytes bounds what a proxy may forward. A client chain is
	// a few KiB; anything larger is rejected before PEM parsing.
	maxForwardedChainBytes = 32 << 10
	// maxForwardedChainLength caps the certificates accepted from one header.
	maxForwardedChainLength = 8
)

// PeerCertificatesFromRequest returns the client certificate chain that proves
// possession of the client key for r.
//
// In-process TLS is the default source. When MTLS_PROXY_CLIENT_CERT_HEADER is
// configured and the direct peer belongs to TRUSTED_PROXY_CIDRS, the named
// header is authoritative instead: that is the explicit opt-in for TLS
// terminated by a reverse proxy. Both conditions are required because a client
// can forge any certificate header whenever TLS terminates upstream.
//
// The operator must make the proxy validate the client certificate and
// overwrite (not append) the header. The last XFCC element wins, so a proxy
// that appends still contributes the element closest to the application.
func PeerCertificatesFromRequest(r *http.Request) []*x509.Certificate {
	if r == nil {
		return nil
	}
	if header := config.MTLSProxyClientCertHeader(); header != "" && config.IsTrustedProxyRequest(r) {
		if values := r.Header.Values(header); len(values) > 0 {
			// A trusted proxy forwarded a chain, so it is authoritative. A
			// malformed value must not fall back to r.TLS: that certificate would
			// belong to the proxy itself, not to the end client.
			return chainFromForwardedHeader(strings.Join(values, ","))
		}
	}
	if r.TLS == nil {
		return nil
	}
	return r.TLS.PeerCertificates
}

// chainFromForwardedHeader accepts both shapes seen in the wild:
//   - an X-Forwarded-Client-Cert element carrying Cert="<escaped PEM>";
//   - the URL-encoded PEM itself (NGINX $ssl_client_escaped_cert and peers).
func chainFromForwardedHeader(raw string) []*x509.Certificate {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxForwardedChainBytes {
		return nil
	}
	if chain := chainFromXFCC(raw); len(chain) > 0 {
		return chain
	}
	return parseForwardedChain(strings.Trim(raw, `"`))
}

func chainFromXFCC(raw string) []*x509.Certificate {
	elements := splitOutsideQuotes(raw, ',')
	for i := len(elements) - 1; i >= 0; i-- {
		value := xfccCertificateValue(elements[i])
		if value == "" {
			continue
		}
		// Stop at the last element that carries a certificate: earlier elements
		// may be client-supplied when the proxy appends rather than overwrites.
		return parseForwardedChain(value)
	}
	return nil
}

func xfccCertificateValue(element string) string {
	for _, pair := range splitOutsideQuotes(element, ';') {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "Cert") {
			continue
		}
		return unquoteXFCC(value)
	}
	return ""
}

func parseForwardedChain(encoded string) []*x509.Certificate {
	decoded, err := url.PathUnescape(strings.TrimSpace(encoded))
	if err != nil {
		return nil
	}
	var chain []*x509.Certificate
	rest := []byte(decoded)
	for len(chain) < maxForwardedChainLength {
		block, remainder := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = remainder
		if block.Type != "CERTIFICATE" {
			continue
		}
		certificate, parseErr := x509.ParseCertificate(block.Bytes)
		if parseErr != nil {
			// Fail closed on a partially parseable chain rather than authenticating
			// with an arbitrary prefix of it.
			return nil
		}
		chain = append(chain, certificate)
	}
	return chain
}

// splitOutsideQuotes splits on separator while ignoring separators inside
// double-quoted XFCC values. Backslash escapes keep the quote state unchanged.
func splitOutsideQuotes(value string, separator byte) []string {
	parts := make([]string, 0, 4)
	start := 0
	quoted := false
	for i := 0; i < len(value); i++ {
		switch {
		case value[i] == '\\' && quoted && i+1 < len(value):
			i++
		case value[i] == '"':
			quoted = !quoted
		case value[i] == separator && !quoted:
			parts = append(parts, value[start:i])
			start = i + 1
		}
	}
	return append(parts, value[start:])
}

func unquoteXFCC(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
		value = strings.ReplaceAll(value, `\"`, `"`)
		value = strings.ReplaceAll(value, `\\`, `\`)
	}
	return value
}
