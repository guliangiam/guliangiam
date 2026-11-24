package dpop

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AccessTokenHash returns the RFC 9449 §4.3 ath value for an access token:
// base64url(SHA-256(token)) without padding.
func AccessTokenHash(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// ValidateAccessTokenHash checks a DPoP proof's ath against the presented
// access token using constant-time comparison.
func ValidateAccessTokenHash(ath, accessToken string) error {
	if strings.TrimSpace(accessToken) == "" {
		return apperrors.NewInvalidArgument("access token is required for ath validation")
	}
	if strings.TrimSpace(ath) == "" {
		return apperrors.NewInvalidArgument("DPoP proof is missing the ath claim")
	}
	expected := AccessTokenHash(accessToken)
	if subtle.ConstantTimeCompare([]byte(ath), []byte(expected)) != 1 {
		return apperrors.NewInvalidArgument("DPoP proof ath does not match the access token")
	}
	return nil
}

// ValidateResourceProof validates a resource-server-side DPoP proof for a
// request that presents an access token: signature/htm/htu/iat/jti checks are
// delegated to ValidateDPoPProof, then ath is bound to the token.
func ValidateResourceProof(proofJWT, accessToken, method, requestURL string) (*Proof, error) {
	if strings.TrimSpace(proofJWT) == "" {
		return nil, apperrors.NewInvalidArgument("DPoP proof is required")
	}
	proof, err := ValidateDPoPProof(proofJWT, method, requestURL, 60*time.Second)
	if err != nil {
		return nil, err
	}
	if err := ValidateAccessTokenHash(proof.Claims.Ath, accessToken); err != nil {
		return nil, err
	}
	return proof, nil
}
