package auth

import (
	"context"
	"encoding/base64"
	"net/url"
	"testing"
	"time"

	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	providermock "github.com/guliangiam/guliangiam/internal/domain/providers/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func oidcProviderFixture() *providerdomain.Provider {
	return &providerdomain.Provider{
		ID:           "provider-oidc",
		Name:         "corporate-oidc",
		Category:     providerdomain.CategoryOIDC,
		ProviderType: providerdomain.ProviderOIDC,
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		AuthUrl:      "https://idp.example.com/authorize",
		TokenUrl:     "https://idp.example.com/token",
		UserInfoUrl:  "https://idp.example.com/userinfo",
		CallbackUrl:  "https://iam.example.com/callback",
		Scopes:       []string{"openid", "profile"},
		Enabled:      true,
	}
}

func TestGenerateSocialOIDCValues(t *testing.T) {
	nonce, verifier, challenge, err := generateSocialOIDCValues()
	require.NoError(t, err)

	require.NotEmpty(t, nonce)
	require.Len(t, verifier, 43, "RFC 7636 verifier is 32 random bytes base64url encoded")
	require.Len(t, challenge, 43, "S256 challenge is a SHA-256 digest base64url encoded")
	assert.Equal(t, buildPKCEChallenge(verifier), challenge)

	decodedVerifier, err := base64.RawURLEncoding.DecodeString(verifier)
	require.NoError(t, err)
	require.Len(t, decodedVerifier, 32)

	secondNonce, secondVerifier, _, err := generateSocialOIDCValues()
	require.NoError(t, err)
	assert.NotEqual(t, nonce, secondNonce)
	assert.NotEqual(t, verifier, secondVerifier)
}

func TestBuildPKCEChallengeRFC7636Vector(t *testing.T) {
	// RFC 7636 Appendix B: verifier -> S256 challenge.
	assert.Equal(t,
		"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
		buildPKCEChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"),
	)
}

func TestSocialOIDCStateStoreConsumeOnce(t *testing.T) {
	store := newSocialOIDCStateStore()
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	store.save(socialOIDCState{
		State:        "state-1",
		ProviderID:   "provider-oidc",
		Nonce:        "nonce-1",
		CodeVerifier: "verifier-1",
		ExpiresAt:    now.Add(socialOIDCStateTTL),
	}, now)

	entry, err := store.consume("state-1", now)
	require.NoError(t, err)
	assert.Equal(t, "provider-oidc", entry.ProviderID)
	assert.Equal(t, "nonce-1", entry.Nonce)
	assert.Equal(t, "verifier-1", entry.CodeVerifier)

	// A second callback with the same state is a replay and must be refused.
	_, err = store.consume("state-1", now)
	require.Error(t, err)
	assert.Equal(t, 401, appErrorStatus(t, err))
}

func TestSocialOIDCStateStoreRejectsExpiredEntry(t *testing.T) {
	store := newSocialOIDCStateStore()
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	store.save(socialOIDCState{
		State:        "state-expired",
		ProviderID:   "provider-oidc",
		Nonce:        "nonce-1",
		CodeVerifier: "verifier-1",
		ExpiresAt:    now.Add(-time.Second),
	}, now.Add(-time.Minute))

	_, err := store.consume("state-expired", now)
	require.Error(t, err)
	assert.Equal(t, 401, appErrorStatus(t, err))
}

func TestSocialProviderConfigForLoginUsesStoredOIDCState(t *testing.T) {
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	store := newSocialOIDCStateStore()
	provider := oidcProviderFixture()
	store.save(socialOIDCState{
		State:        "state-1",
		ProviderID:   provider.ID,
		Nonce:        "nonce-1",
		CodeVerifier: "verifier-1",
		ExpiresAt:    now.Add(socialOIDCStateTTL),
	}, now)

	service := Service{Clock: fakeClock{fixedTime: now}, socialOIDCStates: store}
	config, err := service.socialProviderConfigForLogin(provider, provider.CallbackUrl, SocialLoginCommand{
		State: "state-1",
		// A console-supplied verifier must not override the server-issued one.
		CodeVerifier: "console-verifier",
	}, now)
	require.NoError(t, err)
	assert.Equal(t, "nonce-1", config.Nonce)
	assert.Equal(t, "verifier-1", config.CodeVerifier)
}

func TestSocialProviderConfigForLoginRejectsInvalidOIDCState(t *testing.T) {
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	provider := oidcProviderFixture()

	t.Run("missing state", func(t *testing.T) {
		service := Service{Clock: fakeClock{fixedTime: now}, socialOIDCStates: newSocialOIDCStateStore()}
		_, err := service.socialProviderConfigForLogin(provider, provider.CallbackUrl, SocialLoginCommand{}, now)
		require.Error(t, err)
		assert.Equal(t, 401, appErrorStatus(t, err))
	})

	t.Run("unknown or replayed state", func(t *testing.T) {
		store := newSocialOIDCStateStore()
		store.save(socialOIDCState{
			State:        "state-1",
			ProviderID:   provider.ID,
			Nonce:        "nonce-1",
			CodeVerifier: "verifier-1",
			ExpiresAt:    now.Add(socialOIDCStateTTL),
		}, now)
		service := Service{Clock: fakeClock{fixedTime: now}, socialOIDCStates: store}

		_, err := service.socialProviderConfigForLogin(provider, provider.CallbackUrl, SocialLoginCommand{State: "state-1"}, now)
		require.NoError(t, err)

		_, err = service.socialProviderConfigForLogin(provider, provider.CallbackUrl, SocialLoginCommand{State: "state-1"}, now)
		require.Error(t, err)
		assert.Equal(t, 401, appErrorStatus(t, err))
	})

	t.Run("provider mismatch", func(t *testing.T) {
		store := newSocialOIDCStateStore()
		store.save(socialOIDCState{
			State:        "state-1",
			ProviderID:   "another-provider",
			Nonce:        "nonce-1",
			CodeVerifier: "verifier-1",
			ExpiresAt:    now.Add(socialOIDCStateTTL),
		}, now)
		service := Service{Clock: fakeClock{fixedTime: now}, socialOIDCStates: store}

		_, err := service.socialProviderConfigForLogin(provider, provider.CallbackUrl, SocialLoginCommand{State: "state-1"}, now)
		require.Error(t, err)
		assert.Equal(t, 401, appErrorStatus(t, err))
	})
}

func TestSocialProviderConfigForLoginKeepsConsoleVerifierForOAuth(t *testing.T) {
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	service := Service{Clock: fakeClock{fixedTime: now}, socialOIDCStates: newSocialOIDCStateStore()}
	provider := &providerdomain.Provider{
		ID:           "provider-github",
		Name:         "github",
		Category:     providerdomain.CategoryOAuth,
		ProviderType: providerdomain.ProviderGitHub,
		ClientID:     "client-id",
		AuthUrl:      "https://github.com/login/oauth/authorize",
		TokenUrl:     "https://github.com/login/oauth/access_token",
		CallbackUrl:  "https://iam.example.com/callback",
		Enabled:      true,
	}

	config, err := service.socialProviderConfigForLogin(provider, provider.CallbackUrl, SocialLoginCommand{
		State:        "client-state",
		CodeVerifier: "console-verifier",
	}, now)
	require.NoError(t, err)
	assert.Equal(t, "console-verifier", config.CodeVerifier)
	assert.Empty(t, config.Nonce)
	assert.Empty(t, config.CodeChallenge)
}

func TestGetSocialRedirectURLBindsOIDCNonceAndPKCEToState(t *testing.T) {
	ctrl := gomock.NewController(t)
	uowMock := uowmock.NewMockAuthUoW(ctrl)
	repo := providermock.NewMockRepository(ctrl)
	provider := oidcProviderFixture()
	uowMock.EXPECT().Providers().Return(repo)
	repo.EXPECT().GetByID(gomock.Any(), provider.ID).Return(provider, nil)

	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	service := Service{
		Clock:            fakeClock{fixedTime: now},
		UOW:              uowMock,
		socialOIDCStates: newSocialOIDCStateStore(),
	}

	var captured SocialProviderConfig
	previous := getSocialRedirectURL
	getSocialRedirectURL = func(_ context.Context, config SocialProviderConfig, state string) (string, error) {
		captured = config
		query := url.Values{}
		query.Set("state", state)
		query.Set("nonce", config.Nonce)
		query.Set("code_challenge", config.CodeChallenge)
		query.Set("code_challenge_method", config.CodeChallengeMethod)
		return "https://idp.example.com/authorize?" + query.Encode(), nil
	}
	t.Cleanup(func() { getSocialRedirectURL = previous })

	redirectURL, err := service.GetSocialRedirectURL(context.Background(), SocialAuthorizeCommand{
		ProviderID: provider.ID,
		State:      "state-1",
	})
	require.NoError(t, err)

	parsed, err := url.Parse(redirectURL)
	require.NoError(t, err)
	query := parsed.Query()
	assert.Equal(t, "state-1", query.Get("state"))
	assert.NotEmpty(t, query.Get("nonce"))
	assert.NotEmpty(t, query.Get("code_challenge"))
	assert.Equal(t, "S256", query.Get("code_challenge_method"))
	assert.Equal(t, captured.Nonce, query.Get("nonce"))

	entry, err := service.socialOIDCStateStore().consume("state-1", now)
	require.NoError(t, err)
	assert.Equal(t, captured.Nonce, entry.Nonce)
	assert.Equal(t, buildPKCEChallenge(entry.CodeVerifier), captured.CodeChallenge)
}

func TestGetSocialRedirectURLKeepsOAuthUnchanged(t *testing.T) {
	ctrl := gomock.NewController(t)
	uowMock := uowmock.NewMockAuthUoW(ctrl)
	repo := providermock.NewMockRepository(ctrl)
	provider := &providerdomain.Provider{
		ID:           "provider-github",
		Name:         "github",
		Category:     providerdomain.CategoryOAuth,
		ProviderType: providerdomain.ProviderGitHub,
		ClientID:     "client-id",
		AuthUrl:      "https://github.com/login/oauth/authorize",
		TokenUrl:     "https://github.com/login/oauth/access_token",
		CallbackUrl:  "https://iam.example.com/callback",
		Enabled:      true,
	}
	uowMock.EXPECT().Providers().Return(repo)
	repo.EXPECT().GetByID(gomock.Any(), provider.ID).Return(provider, nil)

	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	service := Service{
		Clock:            fakeClock{fixedTime: now},
		UOW:              uowMock,
		socialOIDCStates: newSocialOIDCStateStore(),
	}

	var captured SocialProviderConfig
	previous := getSocialRedirectURL
	getSocialRedirectURL = func(_ context.Context, config SocialProviderConfig, state string) (string, error) {
		captured = config
		return "https://github.com/login/oauth/authorize?state=" + url.QueryEscape(state), nil
	}
	t.Cleanup(func() { getSocialRedirectURL = previous })

	_, err := service.GetSocialRedirectURL(context.Background(), SocialAuthorizeCommand{
		ProviderID: provider.ID,
		State:      "state-2",
	})
	require.NoError(t, err)
	assert.Empty(t, captured.Nonce)
	assert.Empty(t, captured.CodeChallenge)
	assert.Empty(t, captured.CodeChallengeMethod)

	_, err = service.socialOIDCStateStore().consume("state-2", now)
	require.Error(t, err)
}
