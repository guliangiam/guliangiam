package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

type fakeClientJWKSFetcher struct {
	keys []*applicationdomain.ClientJWK
	err  error
	uri  string
}

func (f *fakeClientJWKSFetcher) Fetch(_ context.Context, jwksURI string) ([]*applicationdomain.ClientJWK, error) {
	f.uri = jwksURI
	return f.keys, f.err
}

func (r *loginTestAppRepository) SetClientJWKSURI(_ context.Context, organizationID, applicationID, jwksURI string, _ time.Time) error {
	if r.jwksURIs == nil {
		r.jwksURIs = map[string]string{}
	}
	r.jwksURIs[organizationID+"/"+applicationID] = jwksURI
	return nil
}

func (r *loginTestAppRepository) GetClientJWKSURI(_ context.Context, organizationID, applicationID string) (*applicationdomain.ClientJWKSURI, error) {
	if uri, ok := r.jwksURIs[organizationID+"/"+applicationID]; ok {
		return &applicationdomain.ClientJWKSURI{ApplicationID: applicationID, OrganizationID: organizationID, JWKSURI: uri}, nil
	}
	return nil, errors.New("client jwks_uri not found")
}

func (r *loginTestAppRepository) DeleteClientJWKSURI(_ context.Context, organizationID, applicationID string) error {
	delete(r.jwksURIs, organizationID+"/"+applicationID)
	return nil
}

func TestVerifyPrivateKeyJWTAssertion_FetchesRemoteJWKSURI(t *testing.T) {
	svc, repo, app, uow := newAssertionService(t)
	rsaKey := testAssertionRSAKey(t)
	repo.jwksURIs = map[string]string{
		testOrgID + "/" + testAppID: "https://client.example.com/jwks.json",
	}
	fetcher := &fakeClientJWKSFetcher{keys: []*applicationdomain.ClientJWK{{
		Kid: testAppJWKKid, Alg: "RS256", JWK: rsaPublicJWKJSON(t, rsaKey, testAppJWKKid, "RS256"),
	}}}
	svc.ClientJWKSFetcher = fetcher

	now := svc.Clock.Now().UTC()
	assertion := signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(time.Minute), nil), rsaKey)
	require.NoError(t, svc.verifyPrivateKeyJWTAssertion(context.Background(), uow, app, assertion, ClientAssertionTypeJWT, testTokenEndpointURL))
	require.Equal(t, "https://client.example.com/jwks.json", fetcher.uri)
}

func TestVerifyPrivateKeyJWTAssertion_RemoteFetchFailureIsRejected(t *testing.T) {
	svc, repo, app, uow := newAssertionService(t)
	rsaKey := testAssertionRSAKey(t)
	repo.jwksURIs = map[string]string{testOrgID + "/" + testAppID: "https://client.example.com/jwks.json"}
	svc.ClientJWKSFetcher = &fakeClientJWKSFetcher{err: errors.New("network down")}

	now := svc.Clock.Now().UTC()
	assertion := signClientAssertion(t, "RS256", testAppJWKKid, assertionClaims(app.ClientID, now.Add(time.Minute), nil), rsaKey)
	err := svc.verifyPrivateKeyJWTAssertion(context.Background(), uow, app, assertion, ClientAssertionTypeJWT, testTokenEndpointURL)
	require.Error(t, err)
}
