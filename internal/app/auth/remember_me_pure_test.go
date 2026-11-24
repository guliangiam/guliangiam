package auth

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	rememberme "github.com/guliangiam/guliangiam/internal/domain/rememberme"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRememberMeRepo is a minimal in-memory repository for the pure exchange path.
type fakeRememberMeRepo struct {
	families map[string]*rememberme.Family
	tokens   map[string]*rememberme.Token // keyed by token hash
}

func newFakeRememberMeRepo() *fakeRememberMeRepo {
	return &fakeRememberMeRepo{
		families: map[string]*rememberme.Family{},
		tokens:   map[string]*rememberme.Token{},
	}
}

func (f *fakeRememberMeRepo) CreateFamily(_ context.Context, family *rememberme.Family) error {
	f.families[family.ID] = family
	return nil
}

func (f *fakeRememberMeRepo) GetFamily(_ context.Context, id string) (*rememberme.Family, error) {
	if family, ok := f.families[id]; ok {
		return family, nil
	}
	return nil, apperrors.NewNotFound("remember-me family not found")
}

func (f *fakeRememberMeRepo) RevokeFamily(_ context.Context, id string, at time.Time, reason string) error {
	if family, ok := f.families[id]; ok && family.RevokedAt == nil {
		family.RevokedAt = &at
		family.RevokeReason = &reason
	}
	return nil
}

func (f *fakeRememberMeRepo) RevokeAllForUser(_ context.Context, organizationID, userID string, at time.Time, reason string) (int64, error) {
	var count int64
	for _, family := range f.families {
		if family.OrganizationID == organizationID && family.UserID == userID && family.RevokedAt == nil {
			family.RevokedAt = &at
			family.RevokeReason = &reason
			count++
		}
	}
	return count, nil
}

func (f *fakeRememberMeRepo) CreateToken(_ context.Context, token *rememberme.Token) error {
	f.tokens[token.TokenHash] = token
	return nil
}

func (f *fakeRememberMeRepo) GetTokenByHash(_ context.Context, hash string) (*rememberme.Token, error) {
	if token, ok := f.tokens[hash]; ok {
		return token, nil
	}
	return nil, apperrors.NewNotFound("remember-me token not found")
}

func (f *fakeRememberMeRepo) ConsumeToken(_ context.Context, id string, at time.Time) error {
	for _, token := range f.tokens {
		if token.ID != id {
			continue
		}
		if token.ConsumedAt != nil {
			return apperrors.NewNotFound("remember-me token already consumed")
		}
		token.ConsumedAt = &at
		return nil
	}
	return apperrors.NewNotFound("remember-me token not found")
}

func (f *fakeRememberMeRepo) RevokeTokensByFamily(_ context.Context, familyID string, at time.Time) error {
	for _, token := range f.tokens {
		if token.FamilyID == familyID && token.ConsumedAt == nil {
			token.ConsumedAt = &at
		}
	}
	return nil
}

func TestRememberMeTokenFormatAndHash(t *testing.T) {
	plaintext, hash, prefix, err := generateRememberMeToken()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(plaintext, RememberMeTokenPrefix))
	encoded := strings.TrimPrefix(plaintext, RememberMeTokenPrefix)
	raw, decodeErr := base64.RawURLEncoding.DecodeString(encoded)
	require.NoError(t, decodeErr)
	assert.Len(t, raw, rememberMeTokenRandomBytes)
	assert.Len(t, hash, 64)
	_, hexErr := hex.DecodeString(hash)
	require.NoError(t, hexErr)
	assert.Equal(t, plaintext[:rememberMeTokenPrefixLen], prefix)
	assert.Equal(t, rememberMeTokenHash(plaintext), hash)
	assert.Equal(t, rememberMeTokenHash("  "+plaintext+"  "), hash)
}

func TestRememberMeTTLIsThirtyDays(t *testing.T) {
	assert.Equal(t, 30*24*time.Hour, RememberMeTTL)
}

func TestRememberMeExpiryJudgement(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	revoked := now.Add(-time.Minute)

	family := &rememberme.Family{ExpiresAt: now.Add(time.Minute)}
	assert.True(t, rememberMeFamilyActive(family, now))
	family.ExpiresAt = now
	assert.False(t, rememberMeFamilyActive(family, now))
	family.ExpiresAt = now.Add(time.Minute)
	family.RevokedAt = &revoked
	assert.False(t, rememberMeFamilyActive(family, now))

	token := &rememberme.Token{ExpiresAt: now.Add(time.Minute)}
	assert.True(t, rememberMeTokenUsable(token, now))
	token.ConsumedAt = &revoked
	assert.False(t, rememberMeTokenUsable(token, now))
	token.ConsumedAt = nil
	token.ExpiresAt = now
	assert.False(t, rememberMeTokenUsable(token, now))
}

func TestExchangeRememberMeRotatesToken(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	repo := newFakeRememberMeRepo()
	expiresAt := now.Add(RememberMeTTL)
	family := &rememberme.Family{ID: "family-1", OrganizationID: "org-1", UserID: "user-1", ExpiresAt: expiresAt}
	repo.families[family.ID] = family

	plaintext, hash, prefix, err := generateRememberMeToken()
	require.NoError(t, err)
	repo.tokens[hash] = &rememberme.Token{
		ID: "token-1", FamilyID: family.ID, OrganizationID: "org-1",
		TokenHash: hash, TokenPrefix: prefix, ExpiresAt: expiresAt,
	}

	result, err := exchangeRememberMeToken(context.Background(), repo, plaintext, now, nil)
	require.NoError(t, err)
	assert.Equal(t, family.ID, result.FamilyID)
	assert.Equal(t, "user-1", result.UserID)
	assert.Equal(t, "org-1", result.OrganizationID)
	assert.Equal(t, expiresAt, result.ExpiresAt)
	assert.NotEqual(t, plaintext, result.Token)
	assert.Len(t, repo.tokens, 2)

	old := repo.tokens[hash]
	require.NotNil(t, old.ConsumedAt)
	newToken, ok := repo.tokens[rememberMeTokenHash(result.Token)]
	require.True(t, ok)
	assert.Nil(t, newToken.ConsumedAt)
	assert.Equal(t, family.ExpiresAt, newToken.ExpiresAt)
}

func TestExchangeRememberMeReuseRevokesFamily(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	repo := newFakeRememberMeRepo()
	family := &rememberme.Family{ID: "family-1", OrganizationID: "org-1", UserID: "user-1", ExpiresAt: now.Add(RememberMeTTL)}
	repo.families[family.ID] = family
	consumedAt := now.Add(-time.Minute)
	plaintext, hash, prefix, err := generateRememberMeToken()
	require.NoError(t, err)
	repo.tokens[hash] = &rememberme.Token{
		ID: "token-1", FamilyID: family.ID, OrganizationID: "org-1",
		TokenHash: hash, TokenPrefix: prefix, ConsumedAt: &consumedAt, ExpiresAt: family.ExpiresAt,
	}

	_, err = exchangeRememberMeToken(context.Background(), repo, plaintext, now, nil)
	assertErrorCode(t, err, apperrors.CodeUnauthorized)

	require.NotNil(t, family.RevokedAt)
	require.NotNil(t, family.RevokeReason)
	assert.Equal(t, "reuse_detected", *family.RevokeReason)
}

func TestExchangeRememberMeRejectsExpiredAndRevoked(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

	t.Run("expired token", func(t *testing.T) {
		repo := newFakeRememberMeRepo()
		family := &rememberme.Family{ID: "family-1", OrganizationID: "org-1", UserID: "user-1", ExpiresAt: now.Add(time.Hour)}
		repo.families[family.ID] = family
		plaintext, hash, prefix, err := generateRememberMeToken()
		require.NoError(t, err)
		repo.tokens[hash] = &rememberme.Token{
			ID: "token-1", FamilyID: family.ID, OrganizationID: "org-1",
			TokenHash: hash, TokenPrefix: prefix, ExpiresAt: now,
		}
		_, err = exchangeRememberMeToken(context.Background(), repo, plaintext, now, nil)
		assertErrorCode(t, err, apperrors.CodeUnauthorized)
	})

	t.Run("revoked family", func(t *testing.T) {
		repo := newFakeRememberMeRepo()
		revokedAt := now.Add(-time.Minute)
		family := &rememberme.Family{ID: "family-1", OrganizationID: "org-1", UserID: "user-1", ExpiresAt: now.Add(time.Hour), RevokedAt: &revokedAt}
		repo.families[family.ID] = family
		plaintext, hash, prefix, err := generateRememberMeToken()
		require.NoError(t, err)
		repo.tokens[hash] = &rememberme.Token{
			ID: "token-1", FamilyID: family.ID, OrganizationID: "org-1",
			TokenHash: hash, TokenPrefix: prefix, ExpiresAt: family.ExpiresAt,
		}
		_, err = exchangeRememberMeToken(context.Background(), repo, plaintext, now, nil)
		assertErrorCode(t, err, apperrors.CodeUnauthorized)
	})
}

func TestExchangeRememberMeRejectsCrossTenantAndUnknown(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

	t.Run("cross tenant", func(t *testing.T) {
		repo := newFakeRememberMeRepo()
		family := &rememberme.Family{ID: "family-1", OrganizationID: "org-1", UserID: "user-1", ExpiresAt: now.Add(time.Hour)}
		repo.families[family.ID] = family
		plaintext, hash, prefix, err := generateRememberMeToken()
		require.NoError(t, err)
		repo.tokens[hash] = &rememberme.Token{
			ID: "token-1", FamilyID: family.ID, OrganizationID: "org-2",
			TokenHash: hash, TokenPrefix: prefix, ExpiresAt: family.ExpiresAt,
		}
		_, err = exchangeRememberMeToken(context.Background(), repo, plaintext, now, nil)
		assertErrorCode(t, err, apperrors.CodeUnauthorized)
	})

	t.Run("unknown token", func(t *testing.T) {
		repo := newFakeRememberMeRepo()
		_, err := exchangeRememberMeToken(context.Background(), repo, "rm_unknown", now, nil)
		assertErrorCode(t, err, apperrors.CodeUnauthorized)
	})
}
