package auth

import (
	"strings"

	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
)

// resolveIssuer returns the issuer identifier an outbound token must declare.
//
// Precedence is deliberate: an issuer resolved from the current request wins,
// because that is exactly what the discovery document reports for the same
// deployment; the configured value is the fallback for work that outlives the
// request (the back-channel logout fan-out runs after the response is written).
//
// Any deployment that sets ISSUER_URL gets the same answer from both sources, so
// `id_token.iss == logout_token.iss == discovery.issuer` holds for an arbitrary
// external domain (设计报告 14.8 收敛验收).
func (s *Service) resolveIssuer(requestIssuer string) string {
	if trimmed := strings.TrimRight(strings.TrimSpace(requestIssuer), "/"); trimmed != "" {
		return trimmed
	}
	if configured := platformconfig.ConfiguredIssuerURL(); configured != "" {
		return configured
	}
	return strings.TrimRight(strings.TrimSpace(s.IssuerURL), "/")
}
