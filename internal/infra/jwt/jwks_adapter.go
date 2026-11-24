package jwt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

var _ domainauth.JWKSProvider = (*JWKSProviderAdapter)(nil)

type JWKSProviderAdapter struct {
	manager *platformjwks.Manager
}

func NewJWKSProviderAdapter(manager *platformjwks.Manager) *JWKSProviderAdapter {
	return &JWKSProviderAdapter{manager: manager}
}

func (a *JWKSProviderAdapter) GetKeyPair() (*domainauth.KeyPair, error) {
	kp, err := a.manager.GetKeyPair()
	if err != nil {
		return nil, err
	}
	return &domainauth.KeyPair{
		Kid:       kp.Kid,
		Algorithm: kp.Algorithm,
		Key:       kp.Key,
	}, nil
}

func (a *JWKSProviderAdapter) GetJWKS() (*domainauth.JWKS, error) {
	jwks, err := a.manager.GetJWKS()
	if err != nil {
		return nil, err
	}
	return mapJWKS(jwks), nil
}

func (a *JWKSProviderAdapter) GetGMJWKS() (*domainauth.JWKS, error) {
	jwks, err := a.manager.GetGMJWKS()
	if err != nil {
		return nil, err
	}
	return mapJWKS(jwks), nil
}

func mapJWKS(jwks *platformjwks.JWKS) *domainauth.JWKS {
	keys := make([]domainauth.JWK, len(jwks.Keys))
	for i, k := range jwks.Keys {
		keys[i] = domainauth.JWK{
			Kty: k.Kty, Use: k.Use, Kid: k.Kid, Alg: k.Alg,
			N: k.N, E: k.E, Crv: k.Crv, X: k.X, Y: k.Y,
		}
	}
	return &domainauth.JWKS{Keys: keys}
}

func (a *JWKSProviderAdapter) GetPublicKey() (*rsa.PublicKey, error) {
	kp, err := a.manager.GetKeyPair()
	if err != nil {
		return nil, err
	}
	return &kp.Key.PublicKey, nil
}

func (a *JWKSProviderAdapter) SignToken(claims map[string]any, tokenType string) (string, error) {
	return a.SignTokenWithAlgorithm(claims, tokenType, platformjwks.SigningAlgorithmRS256)
}

// SignTokenWithAlgorithm signs a token with the requested algorithm. RS256 is
// the default for callers without an application context; SM2-SM3 is selected
// only from app_oauth_config.token_signing_method.
func (a *JWKSProviderAdapter) SignTokenWithAlgorithm(claims map[string]any, tokenType, algorithm string) (string, error) {
	if algorithm == "" {
		algorithm = platformjwks.SigningAlgorithmRS256
	}
	if err := platformjwks.ValidateSigningAlgorithm(algorithm); err != nil {
		return "", err
	}

	switch algorithm {
	case platformjwks.SigningAlgorithmRS256:
		return a.signRS256(claims, tokenType)
	case platformjwks.SigningAlgorithmSM2SM3:
		return a.signSM2SM3(claims, tokenType)
	default:
		return "", apperrors.NewInvalidArgumentf("signing algorithm %q is allowed by policy but not implemented", algorithm)
	}
}

func (a *JWKSProviderAdapter) signRS256(claims map[string]any, tokenType string) (string, error) {
	kp, err := a.manager.GetKeyPair()
	if err != nil {
		return "", apperrors.WrapInternalf(err, "get signing key")
	}

	signingInput, err := signingInputFor(claims, tokenType, kp.Kid, platformjwks.SigningAlgorithmRS256)
	if err != nil {
		return "", err
	}

	hashed := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, kp.Key, crypto.SHA256, hashed[:])
	if err != nil {
		return "", apperrors.WrapInternalf(err, "sign token")
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (a *JWKSProviderAdapter) signSM2SM3(claims map[string]any, tokenType string) (string, error) {
	kp, err := a.manager.GetGMKeyPair()
	if err != nil {
		return "", apperrors.WrapInternalf(err, "get GM signing key")
	}

	signingInput, err := signingInputFor(claims, tokenType, kp.Kid, platformjwks.SigningAlgorithmSM2SM3)
	if err != nil {
		return "", err
	}

	digest := platformcrypto.SM3Sum([]byte(signingInput))
	signature, err := platformcrypto.NewCryptoSuite().SM2Sign(kp.PrivateKey, digest)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "sign token with SM2-SM3")
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func signingInputFor(claims map[string]any, tokenType, kid, algorithm string) (string, error) {
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
