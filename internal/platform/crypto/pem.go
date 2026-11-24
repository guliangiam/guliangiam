package crypto

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ParseRSAPrivateKeyPEM accepts both PEM encodings merchants actually ship:
// PKCS#8 ("PRIVATE KEY") and PKCS#1 ("RSA PRIVATE KEY").
func ParseRSAPrivateKeyPEM(pemData string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, apperrors.NewInvalidArgument("private key is not valid PEM")
	}

	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, apperrors.NewInvalidArgument("private key is not an RSA key")
		}
		return rsaKey, nil
	}

	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, apperrors.WrapInvalidArgumentf(err, "parse private key")
	}
	return rsaKey, nil
}

// ParseCertificatePEMBundle returns every CERTIFICATE block in a bundle. It is
// the shared parser for gateway credentials and callback verification so both
// sides accept the same PEM shapes.
func ParseCertificatePEMBundle(pemBundle string) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := []byte(pemBundle)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, apperrors.WrapInvalidArgumentf(err, "parse certificate")
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, apperrors.NewInvalidArgument("no CERTIFICATE block found in PEM bundle")
	}
	return certs, nil
}
