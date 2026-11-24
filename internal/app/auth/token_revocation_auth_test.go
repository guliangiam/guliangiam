package auth

import (
	"context"
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

// OPEN-API-64: revocation authenticates private_key_jwt and mTLS clients with
// the same decision tree as the token endpoint.
func TestRevokeToken_AcceptsPrivateKeyJWTAssertion(t *testing.T) {
	svc, repo, app, _ := newAssertionService(t)
	rsaKey := testAssertionRSAKey(t)
	registerClientJWK(t, repo, testOrgID, testAppJWKKid, "RS256", rsaPublicJWKJSON(t, rsaKey, testAppJWKKid, "RS256"))
	base := newServiceTestUow(newTestOrg(), newTestUser(), app)
	base.appRepo = repo
	base.tokenRepo = &testTokenRepository{}
	svc.UOW = base
	now := svc.Clock.Now().UTC()
	assertion := signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(2*time.Minute), nil), rsaKey)

	err := svc.RevokeToken(context.Background(), RevokeTokenCommand{
		Token:               "opaque-token",
		ClientID:            app.ClientID,
		ClientAuthMethod:    applicationdomain.TokenEndpointAuthMethodPrivateKeyJWT,
		ClientAssertion:     assertion,
		ClientAssertionType: ClientAssertionTypeJWT,
		TokenEndpointURL:    testTokenEndpointURL,
	})
	require.NoError(t, err)
}

func TestRevokeToken_RejectsInvalidPrivateKeyJWTAssertion(t *testing.T) {
	svc, _, app, _ := newAssertionService(t)
	err := svc.RevokeToken(context.Background(), RevokeTokenCommand{
		Token:               "opaque-token",
		ClientID:            app.ClientID,
		ClientAuthMethod:    applicationdomain.TokenEndpointAuthMethodPrivateKeyJWT,
		ClientAssertion:     "not-a-jwt",
		ClientAssertionType: ClientAssertionTypeJWT,
		TokenEndpointURL:    testTokenEndpointURL,
	})
	requireInvalidClient(t, err)
}

func TestRevokeToken_AcceptsMTLSClientCertificate(t *testing.T) {
	pki := newMTLSTestPKI(t)
	app := mtlsApplication(applicationdomain.TokenEndpointAuthMethodTLSClientAuth)
	service, _ := mtlsFixture(t, app, []*applicationdomain.ClientCertificate{
		registeredCertificate(testOrgID, applicationdomain.TokenEndpointAuthMethodTLSClientAuth, mustThumbprint(t, pki.leaf)),
	})
	service.TLSClientCAs = pki.roots
	appRepo := &loginTestAppRepository{
		apps:       map[string]*applicationdomain.Application{app.ID: app},
		byClientID: map[string]*applicationdomain.Application{app.ClientID: app},
	}
	base := newServiceTestUow(newTestOrg(), newTestUser(), app)
	base.appRepo = appRepo
	base.tokenRepo = &testTokenRepository{}
	service.UOW = base

	err := service.RevokeToken(context.Background(), RevokeTokenCommand{
		Token:            "opaque-token",
		ClientID:         app.ClientID,
		PeerCertificates: []*x509.Certificate{pki.leaf},
	})
	require.NoError(t, err)
}

func TestRevokeToken_RejectsMTLSWithoutCertificate(t *testing.T) {
	app := mtlsApplication(applicationdomain.TokenEndpointAuthMethodTLSClientAuth)
	service, _ := mtlsFixture(t, app, nil)
	appRepo := &loginTestAppRepository{
		apps:       map[string]*applicationdomain.Application{app.ID: app},
		byClientID: map[string]*applicationdomain.Application{app.ClientID: app},
	}
	base := newServiceTestUow(newTestOrg(), newTestUser(), app)
	base.appRepo = appRepo
	base.tokenRepo = &testTokenRepository{}
	service.UOW = base
	err := service.RevokeToken(context.Background(), RevokeTokenCommand{
		Token:    "opaque-token",
		ClientID: app.ClientID,
	})
	requireInvalidClient(t, err)
}
