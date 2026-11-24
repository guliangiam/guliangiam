package jwt

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

// Issuer signs and validates JWT tokens using RSA keys from the JWKS manager.
type Issuer struct {
	jwks *platformjwks.Manager
}

// NewIssuer creates a new JWT issuer backed by the given JWKS key manager.
func NewIssuer(jwksManager *platformjwks.Manager) *Issuer {
	return &Issuer{jwks: jwksManager}
}

// IssueAccessToken issues a signed JWT access token.
func (i *Issuer) IssueAccessToken(params IssueAccessTokenParams) (string, error) {
	start := time.Now()
	token, err := i.issueAccessTokenInner(params)
	result := "ok"
	if err != nil {
		result = "error"
	}
	signDuration.WithLabelValues("access", result).Observe(time.Since(start).Seconds())
	signTotal.WithLabelValues("access", result).Inc()
	return token, err
}

func (i *Issuer) issueAccessTokenInner(params IssueAccessTokenParams) (string, error) {
	now := time.Now().UTC()
	claims := AccessTokenClaims{
		Issuer:         params.Issuer,
		Subject:        params.Subject,
		Audience:       params.Audience,
		Expiry:         now.Add(params.TTL).Unix(),
		IssuedAt:       now.Unix(),
		JWTID:          params.JWTID,
		Scope:          strings.Join(params.Scope, " "),
		ClientID:       params.ClientID,
		OrganizationID: params.OrganizationID,
		Username:       params.Username,
		Resource:       params.Resource,
	}
	return i.signClaims(claims, params.Algorithm)
}

// IssueIDToken issues a signed OIDC ID token.
func (i *Issuer) IssueIDToken(params IssueIDTokenParams) (string, error) {
	start := time.Now()
	token, err := i.issueIDTokenInner(params)
	result := "ok"
	if err != nil {
		result = "error"
	}
	signDuration.WithLabelValues("id", result).Observe(time.Since(start).Seconds())
	signTotal.WithLabelValues("id", result).Inc()
	return token, err
}

func (i *Issuer) issueIDTokenInner(params IssueIDTokenParams) (string, error) {
	now := time.Now().UTC()
	claims := IDTokenClaims{
		Issuer:             params.Issuer,
		Subject:            params.Subject,
		Audience:           params.Audience,
		Expiry:             now.Add(time.Hour).Unix(),
		IssuedAt:           now.Unix(),
		Nonce:              params.Nonce,
		AccessTokenHash:    params.AccessTokenHash,
		CodeHash:           params.CodeHash,
		AuthenticationTime: params.AuthenticationTime.Unix(),
		SessionID:          params.SessionID,
		JWTID:              params.JWTID,
		Name:               params.Name,
		Username:           params.Username,
		GivenName:          params.GivenName,
		FamilyName:         params.FamilyName,
		MiddleName:         params.MiddleName,
		Nickname:           params.Nickname,
		Email:              params.Email,
		EmailVerified:      params.EmailVerified,
		Phone:              params.Phone,
		PhoneVerified:      params.PhoneVerified,
		Picture:            params.Picture,
		Organization:       params.Organization,
	}
	return i.signClaims(claims, params.Algorithm)
}

// ValidateToken validates a JWT token string and returns its claims.
// It checks the signature, expiry (exp), not-before (nbf), issuer (iss), and audience (aud).
func (i *Issuer) ValidateToken(tokenString string) (map[string]interface{}, error) {
	return i.ValidateTokenWithConfig(tokenString, ValidationConfig{})
}

// ValidationConfig allows callers to specify required issuer and audience for validation.
type ValidationConfig struct {
	// ExpectedIssuer, if set, requires the token's iss claim to match.
	ExpectedIssuer string
	// ExpectedAudience, if set, requires the token's aud claim to match.
	ExpectedAudience string
}

// Supported signing algorithms: RS256 (RSA/SHA-256) and SM2-SM3 (SM2 over an
// SM3 digest). Everything else is a capability rejection, not a policy:
// the platform-wide forbidden/allowed lists live in platform/jwks and are
// applied first (设计报告 14.8, ADR-015, ADR-031).
const (
	signatureAlgorithmRS256 = platformjwks.SigningAlgorithmRS256
	signatureAlgorithmSM2   = platformjwks.SigningAlgorithmSM2SM3
)

// joseHeader is the subset of the JOSE header this verifier inspects.
type joseHeader struct {
	Alg  string `json:"alg"`
	Kid  string `json:"kid"`
	Typ  string `json:"typ"`
	Crit any    `json:"crit"`
}

// decodeJOSEHeader parses the protected header. The header is read before any
// cryptographic work, because the algorithm it names decides which primitive is
// used — and a verifier that picks the primitive first has already lost.
func decodeJOSEHeader(encoded string) (*joseHeader, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("invalid token header encoding")
	}

	var header joseHeader
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&header); err != nil {
		return nil, apperrors.NewInvalidArgument("invalid token header")
	}

	if strings.TrimSpace(header.Alg) == "" {
		return nil, apperrors.NewInvalidArgument("token header is missing alg")
	}
	// Platform policy first: an explicitly forbidden algorithm (`none`, `HS*`)
	// must never reach the capability check below, so the rejection names the
	// real reason instead of "wrong algorithm for this key".
	if err := platformjwks.ValidateSigningAlgorithm(header.Alg); err != nil {
		return nil, err
	}
	if header.Alg != signatureAlgorithmRS256 && header.Alg != signatureAlgorithmSM2 {
		return nil, apperrors.NewInvalidArgument("unsupported token signing algorithm: " + header.Alg)
	}
	// RFC 7515 §4.1.11: a verifier that does not understand every header listed
	// in `crit` must reject the token. This issuer understands none of them.
	if header.Crit != nil {
		return nil, apperrors.NewInvalidArgument("token header contains unsupported critical parameters")
	}
	return &header, nil
}

// ValidateTokenWithConfig validates a JWT token with optional issuer/audience checks.
func (i *Issuer) ValidateTokenWithConfig(tokenString string, cfg ValidationConfig) (map[string]interface{}, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, apperrors.NewInvalidArgument("invalid token format")
	}

	header, err := decodeJOSEHeader(parts[0])
	if err != nil {
		return nil, err
	}

	// Select the verification key by the kid the token presents. The lookup is
	// algorithm-specific: an rsa_ kid must not resolve for SM2-SM3 and vice
	// versa, so a cross-algorithm kid is an unknown-kid rejection, never a
	// fallback to the other key type.
	signingInput := parts[0] + "." + parts[1]
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decode signature")
	}

	switch header.Alg {
	case signatureAlgorithmRS256:
		kp, err := i.jwks.GetKeyPairByID(header.Kid)
		if err != nil {
			return nil, err
		}
		hashed := sha256.Sum256([]byte(signingInput))
		if err := rsa.VerifyPKCS1v15(&kp.Key.PublicKey, crypto.SHA256, hashed[:], sigBytes); err != nil {
			return nil, apperrors.WrapInternalf(err, "invalid signature")
		}
	case signatureAlgorithmSM2:
		kp, err := i.jwks.GetGMKeyPairByID(header.Kid)
		if err != nil {
			return nil, err
		}
		digest := platformcrypto.SM3Sum([]byte(signingInput))
		if !platformcrypto.NewCryptoSuite().SM2Verify(kp.PublicKey, digest, sigBytes) {
			return nil, apperrors.NewUnauthorized("invalid SM2-SM3 signature")
		}
	default:
		return nil, apperrors.NewInvalidArgument("unsupported token signing algorithm: " + header.Alg)
	}

	// Parse payload.
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decode payload")
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse claims")
	}

	now := time.Now().UTC()

	// Check expiry (exp).
	if exp, ok := claims["exp"].(float64); ok {
		if time.Unix(int64(exp), 0).Before(now) {
			return nil, apperrors.NewUnauthorized("token has expired")
		}
	}

	// Check not-before (nbf).
	if nbf, ok := claims["nbf"].(float64); ok {
		if time.Unix(int64(nbf), 0).After(now) {
			return nil, apperrors.NewUnauthorized("token not yet valid")
		}
	}

	// Check issuer (iss) if configured.
	if cfg.ExpectedIssuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != cfg.ExpectedIssuer {
			return nil, apperrors.NewInvalidArgument("invalid token issuer")
		}
	}

	// Check audience (aud) if configured.
	if cfg.ExpectedAudience != "" {
		// aud can be a string or an array of strings.
		switch aud := claims["aud"].(type) {
		case string:
			if aud != cfg.ExpectedAudience {
				return nil, apperrors.NewInvalidArgument("invalid token audience")
			}
		case []interface{}:
			found := false
			for _, a := range aud {
				if s, ok := a.(string); ok && s == cfg.ExpectedAudience {
					found = true
					break
				}
			}
			if !found {
				return nil, apperrors.NewInvalidArgument("invalid token audience")
			}
		default:
			return nil, apperrors.NewInvalidArgument("invalid token audience")
		}
	}

	return claims, nil
}

// signClaims signs the given claims as a JWT with the requested algorithm.
// Empty means RS256, preserving every caller that has no application context.
func (i *Issuer) signClaims(claims interface{}, algorithm string) (string, error) {
	if algorithm == "" {
		algorithm = signatureAlgorithmRS256
	}
	if err := platformjwks.ValidateSigningAlgorithm(algorithm); err != nil {
		return "", err
	}

	switch algorithm {
	case signatureAlgorithmRS256:
		kp, err := i.jwks.GetKeyPair()
		if err != nil {
			return "", apperrors.WrapInternalf(err, "get signing key")
		}
		signingInput, err := issuerSigningInput(claims, "JWT", kp.Kid, algorithm)
		if err != nil {
			return "", err
		}
		hashed := sha256.Sum256([]byte(signingInput))
		signature, err := rsa.SignPKCS1v15(rand.Reader, kp.Key, crypto.SHA256, hashed[:])
		if err != nil {
			return "", apperrors.WrapInternalf(err, "sign token")
		}
		return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
	case signatureAlgorithmSM2:
		kp, err := i.jwks.GetGMKeyPair()
		if err != nil {
			return "", apperrors.WrapInternalf(err, "get GM signing key")
		}
		signingInput, err := issuerSigningInput(claims, "JWT", kp.Kid, algorithm)
		if err != nil {
			return "", err
		}
		signature, err := platformcrypto.NewCryptoSuite().SM2Sign(kp.PrivateKey, platformcrypto.SM3Sum([]byte(signingInput)))
		if err != nil {
			return "", apperrors.WrapInternalf(err, "sign token")
		}
		return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
	default:
		return "", apperrors.NewInvalidArgumentf("signing algorithm %q is allowed by policy but not implemented", algorithm)
	}
}

func issuerSigningInput(claims interface{}, tokenType, kid, algorithm string) (string, error) {
	header := map[string]string{
		"alg": algorithm,
		"typ": tokenType,
		"kid": kid,
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON), nil
}
