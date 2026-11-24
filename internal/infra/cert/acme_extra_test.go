package cert

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcmeUser_GetMethods(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	user := &acmeUser{
		email: "test@example.com",
		key:   key,
	}

	assert.Equal(t, "test@example.com", user.GetEmail())
	assert.Equal(t, key, user.GetPrivateKey())
	assert.Nil(t, user.GetRegistration())
}

func TestGenerateSerialNumber(t *testing.T) {
	sn1 := generateSerialNumber()
	sn2 := generateSerialNumber()
	assert.NotNil(t, sn1)
	assert.NotNil(t, sn2)
	assert.NotEqual(t, sn1, sn2)
}

func TestSelfSignedECCCert(t *testing.T) {
	renewer := NewACMERenewer("test@example.com", true)
	certPEM, keyPEM, err := renewer.selfSignedECCCert("example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, certPEM)
	assert.NotEmpty(t, keyPEM)

	// Verify cert PEM
	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block)
	assert.Equal(t, "CERTIFICATE", block.Type)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	assert.Equal(t, "example.com", cert.Subject.CommonName)
	assert.Contains(t, cert.DNSNames, "example.com")

	// Verify key PEM
	keyBlock, _ := pem.Decode(keyPEM)
	require.NotNil(t, keyBlock)
	assert.Equal(t, "EC PRIVATE KEY", keyBlock.Type)
}

func TestCheckAndRenew_InvalidPEM(t *testing.T) {
	renewer := NewACMERenewer("test@example.com", true)
	_, _, _, err := renewer.CheckAndRenew(context.Background(), "example.com", []byte("not-a-pem"), 30*24*time.Hour)
	assert.Error(t, err)
}

func TestCheckAndRenew_NoRenewalNeeded(t *testing.T) {
	renewer := NewACMERenewer("test@example.com", true)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "example.com"},
		DNSNames:     []string{"example.com"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	_, _, wasRenewed, err := renewer.CheckAndRenew(context.Background(), "example.com", certPEM, 30*24*time.Hour)
	require.NoError(t, err)
	assert.False(t, wasRenewed)
}

func TestNewCertRenewalChecker(t *testing.T) {
	renewer := NewACMERenewer("test@example.com", true)
	checker := NewCertRenewalChecker(renewer, nil, 30*24*time.Hour, 24*time.Hour)
	assert.NotNil(t, checker)
	assert.NotNil(t, checker.stopCh)
}

func TestCertRenewalChecker_Stop(t *testing.T) {
	renewer := NewACMERenewer("test@example.com", true)
	checker := NewCertRenewalChecker(renewer, nil, 30*24*time.Hour, 24*time.Hour)
	checker.Stop()
}

func TestHTTP01Provider_Present(t *testing.T) {
	p := &HTTP01Provider{tokens: make(map[string]string)}
	err := p.Present("example.com", "token-123", "key-auth-456")
	require.NoError(t, err)
	assert.Equal(t, "key-auth-456", p.tokens["token-123"])
}

func TestHTTP01Provider_CleanUp(t *testing.T) {
	p := &HTTP01Provider{tokens: make(map[string]string)}
	p.tokens["token-123"] = "key-auth-456"
	err := p.CleanUp("example.com", "token-123", "key-auth-456")
	require.NoError(t, err)
	_, ok := p.tokens["token-123"]
	assert.False(t, ok)
}
