package certs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCert_Creation(t *testing.T) {
	now := time.Now()
	c := Cert{
		ID:              "cert-001",
		OrganizationID:  "org-001",
		Name:            "tls-cert",
		DisplayName:     "TLS Certificate",
		Scope:           "organization",
		Type:            "x509",
		CryptoAlgorithm: "RSA",
		BitSize:         2048,
		ExpireInYears:   1,
		Certificate:     "-----BEGIN CERTIFICATE-----\nMIIBIjAN...\n-----END CERTIFICATE-----",
		PrivateKey:      "-----BEGIN RSA PRIVATE KEY-----\nMIIEpA...\n-----END RSA PRIVATE KEY-----",
		Domain:          []string{"app.example.com", "api.example.com"},
		CreatedAt:       now,
		UpdatedAt:       now,
		Revision:        1,
	}

	assert.Equal(t, "cert-001", c.ID)
	assert.Equal(t, "RSA", c.CryptoAlgorithm)
	assert.Equal(t, 2048, c.BitSize)
	assert.Len(t, c.Domain, 2)
}

func TestCert_DomainExpiration(t *testing.T) {
	now := time.Now()

	t.Run("domain_not_expired", func(t *testing.T) {
		expireTime := now.Add(365 * 24 * time.Hour)
		c := Cert{DomainExpireTime: &expireTime}
		assert.True(t, now.Before(*c.DomainExpireTime))
	})

	t.Run("domain_expired", func(t *testing.T) {
		expireTime := now.Add(-24 * time.Hour)
		c := Cert{DomainExpireTime: &expireTime}
		assert.True(t, now.After(*c.DomainExpireTime))
	})

	t.Run("no_expiration_info", func(t *testing.T) {
		c := Cert{}
		assert.Nil(t, c.DomainExpireTime)
	})
}

func TestCert_CertificateExpiration(t *testing.T) {
	now := time.Now()

	t.Run("cert_with_expiration", func(t *testing.T) {
		expireTime := now.Add(365 * 24 * time.Hour)
		c := Cert{ExpireTime: &expireTime}
		assert.NotNil(t, c.ExpireTime)
	})
}

// stubRenewer is a test stub for CertificateRenewer interface
type stubRenewer struct {
	obtainErr error
	renewErr  error
}

func (s *stubRenewer) ObtainCertificate(_ context.Context, _ string) (certPEMOut, keyPEMOut []byte, err error) {
	if s.obtainErr != nil {
		return nil, nil, s.obtainErr
	}
	return []byte("cert-pem"), []byte("key-pem"), nil
}

func (s *stubRenewer) RenewCertificate(_ context.Context, _ string, _, _ []byte) (certPEMOut, keyPEMOut []byte, err error) {
	if s.renewErr != nil {
		return nil, nil, s.renewErr
	}
	return []byte("new-cert-pem"), []byte("new-key-pem"), nil
}

func TestCertificateRenewer_Interface(t *testing.T) {
	t.Run("obtain_success", func(t *testing.T) {
		r := &stubRenewer{}
		cert, key, err := r.ObtainCertificate(context.Background(), "example.com")
		assert.NoError(t, err)
		assert.NotEmpty(t, cert)
		assert.NotEmpty(t, key)
	})

	t.Run("renew_success", func(t *testing.T) {
		r := &stubRenewer{}
		cert, key, err := r.RenewCertificate(context.Background(), "example.com", nil, nil)
		assert.NoError(t, err)
		assert.NotEmpty(t, cert)
		assert.NotEmpty(t, key)
	})

	t.Run("obtain_failure", func(t *testing.T) {
		r := &stubRenewer{obtainErr: context.DeadlineExceeded}
		_, _, err := r.ObtainCertificate(context.Background(), "example.com")
		assert.Error(t, err)
	})
}
