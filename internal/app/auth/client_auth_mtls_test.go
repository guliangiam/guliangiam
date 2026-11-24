package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformmtls "github.com/guliangiam/guliangiam/internal/platform/mtls"
)

// mTLS test material is issued for the fixture clock (2026-10-27 08:00 UTC).
var (
	mTLSCertNotBefore = time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	mTLSCertNotAfter  = time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
)

type mtlsPKI struct {
	roots *x509.CertPool
	leaf  *x509.Certificate
}

func newMTLSTestPKI(t *testing.T) mtlsPKI {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "guliangiam test client CA"},
		NotBefore:             mTLSCertNotBefore,
		NotAfter:              mTLSCertNotAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "machine-client"},
		NotBefore:    mTLSCertNotBefore,
		NotAfter:     mTLSCertNotAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(leafDER)
	require.NoError(t, err)

	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	return mtlsPKI{roots: roots, leaf: leaf}
}

// newSelfSignedCert issues a certificate that is its own issuer, exactly what a
// self_signed_tls_client_auth client presents.
func newSelfSignedCert(t *testing.T) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "self-signed-machine-client"},
		NotBefore:    mTLSCertNotBefore,
		NotAfter:     mTLSCertNotAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert
}

func mustThumbprint(t *testing.T, cert *x509.Certificate) string {
	t.Helper()
	thumbprint, err := platformmtls.Thumbprint(cert)
	require.NoError(t, err)
	return thumbprint
}

func mtlsApplication(method string) *applicationdomain.Application {
	return &applicationdomain.Application{
		ID:                      testAppID,
		OrganizationID:          testOrgID,
		Slug:                    "mtls-app",
		ClientID:                "gl_cid_mtls",
		Status:                  "active",
		TokenEndpointAuthMethod: method,
		GrantTypes:              []string{clientCredentialsGrantType},
	}
}

func registeredCertificate(organizationID, method, thumbprint string) *applicationdomain.ClientCertificate {
	notBefore := mTLSCertNotBefore
	notAfter := mTLSCertNotAfter
	return &applicationdomain.ClientCertificate{
		ID:                "cert-1",
		OrganizationID:    organizationID,
		ApplicationID:     testAppID,
		AuthMethod:        string(method),
		FingerprintSHA256: thumbprint,
		Status:            applicationdomain.ClientCertificateStatusActive,
		NotBefore:         &notBefore,
		NotAfter:          &notAfter,
	}
}

// mtlsFixture builds the client_credentials fixture with the given registered
// certificate rows. The fake repository mirrors the SQL predicate, so a lookup
// under a different organization returns nothing.
func mtlsFixture(t *testing.T, app *applicationdomain.Application, certificates []*applicationdomain.ClientCertificate) (Service, *authWLITestUOW) {
	t.Helper()
	fakeAppClientCertificates = map[string][]*applicationdomain.ClientCertificate{}
	for _, certificate := range certificates {
		key := fakeAppClientSecretKey(certificate.OrganizationID, certificate.ApplicationID)
		fakeAppClientCertificates[key] = append(fakeAppClientCertificates[key], certificate)
	}
	service, uow := newClientCredentialsFixture(t, nil, app)
	return service, uow
}

func requireInvalidClient(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var protocolErr apperrors.ProtocolCodeError
	require.ErrorAs(t, err, &protocolErr)
	assert.Equal(t, "invalid_client", protocolErr.ProtocolCode())
	var statusErr interface{ HTTPStatus() int }
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, 401, statusErr.HTTPStatus())
}

// requireUnsupportedGrant asserts that client authentication succeeded and the
// grant itself was refused: no principal type can be persisted as a token
// subject yet, so client_credentials answers unsupported_grant_type (400).
func requireUnsupportedGrant(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var protocolErr apperrors.ProtocolCodeError
	require.ErrorAs(t, err, &protocolErr)
	assert.Equal(t, "unsupported_grant_type", protocolErr.ProtocolCode())
	var statusErr interface{ HTTPStatus() int }
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, 400, statusErr.HTTPStatus())
}

// tls_client_auth: a chain issued by the configured trusted CA plus a
// registered fingerprint authenticates the client.
func TestMTLSClientAuth_TrustedCAChainAndFingerprintAccepted(t *testing.T) {
	pki := newMTLSTestPKI(t)
	app := mtlsApplication(applicationdomain.TokenEndpointAuthMethodTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodTLSClientAuth, mustThumbprint(t, pki.leaf)),
	})
	service.TLSClientCAs = pki.roots

	response, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{pki.leaf},
	})
	assert.Nil(t, response)
	requireUnsupportedGrant(t, err)
}

// A certificate whose fingerprint is not registered is rejected.
func TestMTLSClientAuth_FingerprintMismatchRejected(t *testing.T) {
	pki := newMTLSTestPKI(t)
	app := mtlsApplication(applicationdomain.TokenEndpointAuthMethodTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodTLSClientAuth, mustThumbprint(t, newSelfSignedCert(t))),
	})
	service.TLSClientCAs = pki.roots

	_, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{pki.leaf},
	})
	requireInvalidClient(t, err)
}

// tls_client_auth requires a trusted chain: a self-signed certificate cannot
// satisfy it even when its fingerprint is registered.
func TestMTLSClientAuth_SelfSignedCertificateRejected(t *testing.T) {
	selfSigned := newSelfSignedCert(t)
	app := mtlsApplication(applicationdomain.TokenEndpointAuthMethodTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodTLSClientAuth, mustThumbprint(t, selfSigned)),
	})
	// Trusted CA configured, but the presented certificate is not part of it.
	service.TLSClientCAs = x509.NewCertPool()

	_, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{selfSigned},
	})
	requireInvalidClient(t, err)
}

// tls_client_auth fails closed when no trusted CA is configured.
func TestMTLSClientAuth_NoTrustedCARejected(t *testing.T) {
	pki := newMTLSTestPKI(t)
	app := mtlsApplication(applicationdomain.TokenEndpointAuthMethodTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodTLSClientAuth, mustThumbprint(t, pki.leaf)),
	})
	service.TLSClientCAs = nil

	_, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{pki.leaf},
	})
	requireInvalidClient(t, err)
}

// tls_client_auth rejects a certificate issued by a CA outside the configured
// trust store even though the certificate is present in PeerCertificates and
// its fingerprint is registered for this method: chain verification runs before
// the registration lookup, so an untrusted chain alone fails the request.
func TestMTLSClientAuth_UntrustedCAIssuedCertificateRejected(t *testing.T) {
	trustedPKI := newMTLSTestPKI(t)
	untrustedPKI := newMTLSTestPKI(t)
	app := mtlsApplication(applicationdomain.TokenEndpointAuthMethodTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodTLSClientAuth, mustThumbprint(t, untrustedPKI.leaf)),
	})
	// A real trust store is configured; it just does not contain the issuing CA.
	service.TLSClientCAs = trustedPKI.roots

	_, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{untrustedPKI.leaf},
	})
	requireInvalidClient(t, err)
}

// self_signed_tls_client_auth matches the registered fingerprint byte-for-byte
// and does not require any chain.
func TestSelfSignedMTLS_FingerprintAccepted(t *testing.T) {
	selfSigned := newSelfSignedCert(t)
	app := mtlsApplication(applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth, mustThumbprint(t, selfSigned)),
	})

	response, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{selfSigned},
	})
	assert.Nil(t, response)
	requireUnsupportedGrant(t, err)
}

// A row registered for self_signed_tls_client_auth cannot satisfy
// tls_client_auth (and the reverse).
func TestMTLS_RowRegisteredForOtherMethodRejected(t *testing.T) {
	pki := newMTLSTestPKI(t)
	app := mtlsApplication(applicationdomain.TokenEndpointAuthMethodTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth, mustThumbprint(t, pki.leaf)),
	})
	service.TLSClientCAs = pki.roots

	_, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{pki.leaf},
	})
	requireInvalidClient(t, err)
}

// No peer certificate on the connection (r.TLS nil upstream) is rejected.
func TestMTLS_NoPeerCertificateRejected(t *testing.T) {
	app := mtlsApplication(applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth, mustThumbprint(t, newSelfSignedCert(t))),
	})

	_, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID: app.ClientID,
	})
	requireInvalidClient(t, err)
}

// A certificate row owned by another organization is invisible: a B-tenant
// application cannot authenticate with an A-tenant registration.
func TestMTLS_CrossTenantCertificateRowRejected(t *testing.T) {
	selfSigned := newSelfSignedCert(t)
	app := mtlsApplication(applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth)
	app.OrganizationID = "org-b"
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate("org-a", applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth, mustThumbprint(t, selfSigned)),
	})

	_, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{selfSigned},
	})
	requireInvalidClient(t, err)
}

// A client_secret_basic application cannot authenticate via mTLS.
func TestMTLS_MethodMismatchWithSecretRejected(t *testing.T) {
	pki := newMTLSTestPKI(t)
	app := mtlsApplication(applicationdomain.TokenEndpointAuthMethodClientSecretBasic)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodTLSClientAuth, mustThumbprint(t, pki.leaf)),
	})
	service.TLSClientCAs = pki.roots

	_, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{pki.leaf},
	})
	requireInvalidClient(t, err)
}

// An mTLS failure is indistinguishable from an unknown client: identical code,
// status and message.
func TestMTLS_ErrorMatchesUnknownClient(t *testing.T) {
	selfSigned := newSelfSignedCert(t)
	app := mtlsApplication(applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth, mustThumbprint(t, newSelfSignedCert(t))),
	})

	_, mismatchErr := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{selfSigned},
	})
	_, unknownErr := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID:         "gl_cid_unknown",
		PeerCertificates: []*x509.Certificate{selfSigned},
	})
	requireInvalidClient(t, mismatchErr)
	requireInvalidClient(t, unknownErr)
	assert.Equal(t, unknownErr.Error(), mismatchErr.Error())
}
