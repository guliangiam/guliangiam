package auth

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// parseJWTClaimsUnverified decodes a JWT payload without checking the
// signature. It is only used to locate the key/issuer a token claims; any
// security decision must re-verify the signature.
func parseJWTClaimsUnverified(tokenString string) (map[string]interface{}, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, apperrors.NewInvalidArgument("invalid token format")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, apperrors.WrapInternal(err, "decode payload")
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, apperrors.WrapInternal(err, "parse claims")
	}

	return claims, nil
}

// verifyJWTSignature verifies the RS256 signature of a JWT using the given
// public key.
func verifyJWTSignature(tokenString string, publicKey *rsa.PublicKey) error {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return apperrors.NewInvalidArgument("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return apperrors.WrapInternal(err, "decode signature")
	}

	hashed := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], sigBytes); err != nil {
		return apperrors.WrapInternal(err, "invalid signature")
	}

	return nil
}

// parseRSAPublicKeyFromPEM parses an RSA public key from a PEM-encoded
// certificate or public key block.
func parseRSAPublicKeyFromPEM(pemData string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, apperrors.NewInvalidArgument("invalid PEM")
	}

	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, apperrors.NewInvalidArgument("certificate does not contain an RSA public key")
		}
		return publicKey, nil
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		rsaPublicKey, pkcs1Err := x509.ParsePKCS1PublicKey(block.Bytes)
		if pkcs1Err != nil {
			return nil, apperrors.WrapInternal(err, "parse public key")
		}
		return rsaPublicKey, nil
	}
	rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, apperrors.NewInvalidArgument("public key is not RSA")
	}
	return rsaPublicKey, nil
}
