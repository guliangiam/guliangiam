package wellknown

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"testing"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appkeys "github.com/guliangiam/guliangiam/internal/app/keys"
	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domainkeys "github.com/guliangiam/guliangiam/internal/domain/keys"
	keysmock "github.com/guliangiam/guliangiam/internal/domain/keys/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type staticJWKSProvider struct {
	set *appkeys.JWKS
	err error
}

func (p staticJWKSProvider) GetJWKS() (*appkeys.JWKS, error) {
	return p.set, p.err
}

func (p staticJWKSProvider) GetGMJWKS() (*appkeys.JWKS, error) {
	return p.set, p.err
}

func keyServiceWithKeys(t *testing.T, orgID string, keys ...*domainkeys.Key) *appkeys.Service {
	t.Helper()

	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockAuthUoW(ctrl)
	repo := keysmock.NewMockRepository(ctrl)
	uow.EXPECT().Keys().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domainkeys.ListQuery) ([]*domainkeys.Key, int64, error) {
			matched := make([]*domainkeys.Key, 0, 1)
			for _, key := range keys {
				if key.Name == query.Keyword && key.OrganizationID == orgID {
					matched = append(matched, key)
				}
			}
			return matched, int64(len(matched)), nil
		}).AnyTimes()
	return &appkeys.Service{UOW: uow}
}

// GAP-007: an unwired key service is a dependency failure, not a reason to hand
// out the platform keys under the application URL.
func TestAppJWKS_MissingKeyServiceIs503(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, Handler{OIDCDiscoveryFetch: &mockOIDCDiscoveryFetcher{
		application: &appauth.OIDCApplication{Slug: "app1", OrganizationID: "org-1"},
	}})
	resp := doRequest(t, router, http.MethodGet, "/.well-known/app1/jwks.json", nil)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestAppJWKS_UnknownApplicationIs404(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, Handler{
		JWKSManager:        staticJWKSProvider{set: &appkeys.JWKS{}},
		OIDCDiscoveryFetch: &mockOIDCDiscoveryFetcher{resolveErr: apperrors.NewNotFound("application not found")},
		KeyService:         keyServiceWithKeys(t, "org-1"),
	})
	resp := doRequest(t, router, http.MethodGet, "/.well-known/app1/jwks.json", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAppJWKS_ApplicationLookupFailureIs503(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, Handler{
		JWKSManager:        staticJWKSProvider{set: &appkeys.JWKS{}},
		OIDCDiscoveryFetch: &mockOIDCDiscoveryFetcher{resolveErr: errors.New("database unavailable")},
		KeyService:         keyServiceWithKeys(t, "org-1"),
	})
	resp := doRequest(t, router, http.MethodGet, "/.well-known/app1/jwks.json", nil)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestAppJWKS_MissingApplicationKeyIs404(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, Handler{
		JWKSManager:        staticJWKSProvider{set: &appkeys.JWKS{}},
		OIDCDiscoveryFetch: &mockOIDCDiscoveryFetcher{application: &appauth.OIDCApplication{Slug: "app1", OrganizationID: "org-1"}},
		KeyService:         keyServiceWithKeys(t, "org-1"),
	})
	resp := doRequest(t, router, http.MethodGet, "/.well-known/app1/jwks.json", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAppJWKS_UnparseableKeyIs404(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, Handler{
		JWKSManager:        staticJWKSProvider{set: &appkeys.JWKS{}},
		OIDCDiscoveryFetch: &mockOIDCDiscoveryFetcher{application: &appauth.OIDCApplication{Slug: "app1", OrganizationID: "org-1"}},
		KeyService: keyServiceWithKeys(t, "org-1", &domainkeys.Key{
			ID: "app-key-1", OrganizationID: "org-1", Name: "app-signing-app1", PublicKey: "not a PEM block",
		}),
	})
	resp := doRequest(t, router, http.MethodGet, "/.well-known/app1/jwks.json", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAppJWKS_PlatformJWKSFailureIs503(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, Handler{
		JWKSManager:        staticJWKSProvider{err: errors.New("platform keys unavailable")},
		OIDCDiscoveryFetch: &mockOIDCDiscoveryFetcher{application: &appauth.OIDCApplication{Slug: "app1", OrganizationID: "org-1"}},
		KeyService: keyServiceWithKeys(t, "org-1", &domainkeys.Key{
			ID: "app-key-1", OrganizationID: "org-1", Name: "app-signing-app1", PublicKey: testRSAPublicKeyPEM(t),
		}),
	})
	resp := doRequest(t, router, http.MethodGet, "/.well-known/app1/jwks.json", nil)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

// AC-CRYPTO-002: the application-scoped entry is an RS256/RSA JWK, so its kid
// carries the rsa_ family prefix. A row id that already carries its family
// prefix is not prefixed twice, and an id carrying the other family's prefix
// cannot leak that prefix into the RSA family.
func TestAppJWKS_ApplicationKeyKidCarriesAlgorithmFamilyPrefix(t *testing.T) {
	cases := []struct {
		name    string
		keyID   string
		wantKid string
	}{
		{"plain row id", "app-key-1", platformjwks.KidPrefixRSA + "app-key-1"},
		{"already prefixed row id", platformjwks.KidPrefixRSA + "app-key-2", platformjwks.KidPrefixRSA + "app-key-2"},
		{"foreign family row id", platformjwks.KidPrefixGM + "app-key-3", platformjwks.KidPrefixRSA + platformjwks.KidPrefixGM + "app-key-3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := testRouter()
			RegisterChiRoutes(router, Handler{
				JWKSManager:        staticJWKSProvider{set: &appkeys.JWKS{}},
				OIDCDiscoveryFetch: &mockOIDCDiscoveryFetcher{application: &appauth.OIDCApplication{Slug: "app1", OrganizationID: "org-1"}},
				KeyService: keyServiceWithKeys(t, "org-1", &domainkeys.Key{
					ID: tc.keyID, OrganizationID: "org-1", Name: "app-signing-app1",
					KeyType: "rsa", Algorithm: platformjwks.SigningAlgorithmRS256, IsEnabled: true,
					PublicKey: testRSAPublicKeyPEM(t),
				}),
			})
			resp := doRequest(t, router, http.MethodGet, "/.well-known/app1/jwks.json", nil)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			var set appkeys.JWKS
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&set))
			require.NoError(t, resp.Body.Close())
			require.Len(t, set.Keys, 1)
			assert.Equal(t, tc.wantKid, set.Keys[0].Kid)
			assert.Equal(t, platformjwks.SigningAlgorithmRS256, set.Keys[0].Alg)
			assert.Equal(t, "RSA", set.Keys[0].Kty)
		})
	}
}

// A platform key is canonical. When the application entry repeats its material
// or steals its kid, the platform entry is the one published, so one public key
// never appears under two kids and one kid never maps to two public keys.
func TestAppJWKS_PlatformKeyWinsOnKidOrMaterialOverlap(t *testing.T) {
	platformPEM := testRSAPublicKeyPEM(t)
	platformKey := rsaJWKFromPEM(t, platformPEM, platformjwks.KidPrefixRSA+"platform-1")

	cases := []struct {
		name     string
		appKeyID string
		appPEM   string
	}{
		{"same material different kid", "app-key-1", platformPEM},
		{"same kid different material", "platform-1", testRSAPublicKeyPEM(t)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := testRouter()
			RegisterChiRoutes(router, Handler{
				JWKSManager:        staticJWKSProvider{set: &appkeys.JWKS{Keys: []appkeys.JWK{platformKey}}},
				OIDCDiscoveryFetch: &mockOIDCDiscoveryFetcher{application: &appauth.OIDCApplication{Slug: "app1", OrganizationID: "org-1"}},
				KeyService: keyServiceWithKeys(t, "org-1", &domainkeys.Key{
					ID: tc.appKeyID, OrganizationID: "org-1", Name: "app-signing-app1",
					KeyType: "rsa", Algorithm: platformjwks.SigningAlgorithmRS256, IsEnabled: true,
					PublicKey: tc.appPEM,
				}),
			})
			resp := doRequest(t, router, http.MethodGet, "/.well-known/app1/jwks.json", nil)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			var set appkeys.JWKS
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&set))
			require.NoError(t, resp.Body.Close())
			require.Len(t, set.Keys, 1)
			assert.Equal(t, platformKey.Kid, set.Keys[0].Kid)
			assert.Equal(t, platformKey.N, set.Keys[0].N)
			assert.Equal(t, platformKey.E, set.Keys[0].E)
		})
	}
}

func rsaJWKFromPEM(t *testing.T, pemStr, kid string) appkeys.JWK {
	t.Helper()
	pub, err := parseRSAPublicKeyPEM(pemStr)
	require.NoError(t, err)
	return appkeys.JWK{
		Kty: "RSA",
		Use: "sig",
		Kid: kid,
		Alg: platformjwks.SigningAlgorithmRS256,
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}
