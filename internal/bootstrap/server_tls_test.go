package bootstrap

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestCA(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	return path
}

func TestProductionTLSConfig_OptionalClientCertificates(t *testing.T) {
	t.Setenv("TLS_CLIENT_CA_FILE", writeTestCA(t))
	cfg, err := productionTLSConfig()
	require.NoError(t, err)
	assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
	// Client certificates are always requested but never verified by the TLS
	// stack: self_signed_tls_client_auth clients have no trusted chain, so
	// tls_client_auth verification happens in the app layer against this pool.
	assert.Equal(t, tls.RequestClientCert, cfg.ClientAuth)
	require.NotNil(t, cfg.ClientCAs)
}

func TestProductionTLSConfig_ClientCertificatesRequestedWithoutCA(t *testing.T) {
	t.Setenv("TLS_CLIENT_CA_FILE", "")
	cfg, err := productionTLSConfig()
	require.NoError(t, err)
	assert.Equal(t, tls.RequestClientCert, cfg.ClientAuth)
	assert.Nil(t, cfg.ClientCAs, "no trusted CA means tls_client_auth fails closed at the app layer")
}

func TestProductionTLSConfig_InvalidClientCA(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pem")
	require.NoError(t, os.WriteFile(path, []byte("not a certificate"), 0o600))
	t.Setenv("TLS_CLIENT_CA_FILE", path)
	_, err := productionTLSConfig()
	require.Error(t, err)
}
