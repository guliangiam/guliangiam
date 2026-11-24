package auth

import (
	"context"
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

// authorizeCodeMTLSApplication mirrors mtlsApplication but registers the
// authorization_code grant, so a certificate can only pass the client-auth
// gate (the grant itself is checked by the caller before this function).
func authorizeCodeMTLSApplication(method string) *applicationdomain.Application {
	app := mtlsApplication(method)
	app.GrantTypes = []string{grantAuthorizationCode}
	return app
}

func runAuthorizationCodeClientAuth(service Service, app *applicationdomain.Application, command ExchangeAuthorizationCodeCommand) error {
	return service.UOW.WithinTransaction(context.Background(), func(ctx context.Context, tx appuow.UnitOfWork) error {
		return service.validateAuthorizationCodeClientAuthentication(ctx, tx, command, app)
	})
}

// An authorization-code client registered for tls_client_auth authenticates
// with the TLS chain + registered fingerprint. Client authentication is
// orthogonal to the grant type; this path used to reject every mTLS method as
// unimplemented.
func TestAuthorizationCodeMTLS_TrustedChainAndFingerprintAccepted(t *testing.T) {
	pki := newMTLSTestPKI(t)
	app := authorizeCodeMTLSApplication(applicationdomain.TokenEndpointAuthMethodTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodTLSClientAuth, mustThumbprint(t, pki.leaf)),
	})
	service.TLSClientCAs = pki.roots

	err := runAuthorizationCodeClientAuth(service, app, ExchangeAuthorizationCodeCommand{
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{pki.leaf},
	})
	require.NoError(t, err)
}

// The same grant with a certificate whose fingerprint is not registered must
// fail with invalid_client / 401.
func TestAuthorizationCodeMTLS_FingerprintMismatchRejected(t *testing.T) {
	pki := newMTLSTestPKI(t)
	app := authorizeCodeMTLSApplication(applicationdomain.TokenEndpointAuthMethodTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodTLSClientAuth, mustThumbprint(t, newSelfSignedCert(t))),
	})
	service.TLSClientCAs = pki.roots

	err := runAuthorizationCodeClientAuth(service, app, ExchangeAuthorizationCodeCommand{
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{pki.leaf},
	})
	requireInvalidClient(t, err)
}

// No peer certificate means no mTLS credential at all: rejected, never treated
// as a public client.
func TestAuthorizationCodeMTLS_NoPeerCertificateRejected(t *testing.T) {
	app := authorizeCodeMTLSApplication(applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth, mustThumbprint(t, newSelfSignedCert(t))),
	})

	err := runAuthorizationCodeClientAuth(service, app, ExchangeAuthorizationCodeCommand{
		ClientID: app.ClientID,
	})
	requireInvalidClient(t, err)
}

// A secret presented by an mTLS client is a method mismatch, not a fallback.
func TestAuthorizationCodeMTLS_SecretPresentedRejected(t *testing.T) {
	selfSigned := newSelfSignedCert(t)
	app := authorizeCodeMTLSApplication(applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth, mustThumbprint(t, selfSigned)),
	})

	err := runAuthorizationCodeClientAuth(service, app, ExchangeAuthorizationCodeCommand{
		ClientID:     app.ClientID,
		ClientSecret: "not-a-certificate",
		PeerCertificates: []*x509.Certificate{
			selfSigned,
		},
	})
	requireInvalidClient(t, err)
}

// self_signed_tls_client_auth needs no chain but still needs the registered
// fingerprint for the same method.
func TestAuthorizationCodeMTLS_SelfSignedAccepted(t *testing.T) {
	selfSigned := newSelfSignedCert(t)
	app := authorizeCodeMTLSApplication(applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth, mustThumbprint(t, selfSigned)),
	})

	err := runAuthorizationCodeClientAuth(service, app, ExchangeAuthorizationCodeCommand{
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{selfSigned},
	})
	require.NoError(t, err)
}
