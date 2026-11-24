package auth

import (
	"crypto/rsa"
)

type JWKSProvider interface {
	GetKeyPair() (*KeyPair, error)
	// GetJWKS publishes the standard-algorithm (RSA/ES) public key set only.
	GetJWKS() (*JWKS, error)
	// GetGMJWKS publishes the 国密 SM2-SM3 public key set only, for the dedicated
	// GM endpoint consumed by GM-capable relying parties.
	GetGMJWKS() (*JWKS, error)
	GetPublicKey() (*rsa.PublicKey, error)
	// SignToken signs with the platform default algorithm (RS256). Callers that
	// hold an application's token_signing_method must use SignTokenWithAlgorithm
	// so one application never mixes algorithm families.
	SignToken(claims map[string]any, tokenType string) (string, error)
	// SignTokenWithAlgorithm signs with the requested JOSE algorithm. An empty
	// algorithm falls back to RS256; values outside the platform allow-list are
	// rejected rather than downgraded.
	SignTokenWithAlgorithm(claims map[string]any, tokenType, algorithm string) (string, error)
}

type KeyPair struct {
	Kid       string
	Algorithm string
	Key       *rsa.PrivateKey
}

type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

type JWKS struct {
	Keys []JWK `json:"keys"`
}
