package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	apptokens "github.com/guliangiam/guliangiam/internal/app/tokens"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type pgRefreshExchanger struct{ tokens apptokens.Service }

func (a pgRefreshExchanger) ExchangeRefreshToken(ctx context.Context, request RefreshTokenExchangeRequest) (RefreshTokenExchangeResult, error) {
	issued, err := a.tokens.RefreshAccessToken(ctx, apptokens.RefreshAccessTokenCommand{
		OrganizationID: request.OrganizationID,
		ApplicationID:  request.ApplicationID,
		RefreshToken:   request.RefreshToken,
	})
	if err != nil {
		return RefreshTokenExchangeResult{}, err
	}
	return RefreshTokenExchangeResult{
		AccessToken: issued.TokenValue, RefreshToken: issued.RefreshToken,
		ExpiresAt: issued.ExpiresAt, Scopes: issued.Scopes, TokenType: "Bearer",
	}, nil
}

func tokenHashForTest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// TestExchangeRefreshTokenPostgres is the real-PostgreSQL evidence for
// OPEN-API-63: the refresh_token grant authenticates the client, rotates the
// token pair and revokes the old refresh token.
func TestExchangeRefreshTokenPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the refresh_token grant test")
	}
	// Organization reads decrypt confidential fields through the global field
	// encryptor; the refresh grant needs a test-only 32-byte key.
	if os.Getenv("ENCRYPTION_KEY_CURRENT") == "" {
		os.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x5a}, 32)))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var orgID, userID, appID, clientID string
	require.NoError(t, pool.QueryRow(ctx, "SELECT a.organization_id::text, u.id::text, a.id::text, a.client_id FROM guliangiam.applications a JOIN guliangiam.users u ON u.organization_id = a.organization_id JOIN guliangiam.organizations o ON o.id = a.organization_id WHERE a.slug = 'app-built-in' AND o.slug = 'built-in' AND u.username = 'admin' LIMIT 1").Scan(&orgID, &userID, &appID, &clientID))

	accessID, refreshID := uuid.NewString(), uuid.NewString()
	oldAccess, oldRefresh := "gl_pat_pg_"+uuid.NewString(), "gl_prt_pg_"+uuid.NewString()
	now := time.Now().UTC()
	exec := func(query string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, query, args...)
		require.NoError(t, err)
	}
	exec("DELETE FROM guliangiam.refresh_tokens WHERE token_hash = $1", tokenHashForTest(oldRefresh))
	exec("DELETE FROM guliangiam.access_tokens WHERE token_hash = $1", tokenHashForTest(oldAccess))
	exec("INSERT INTO guliangiam.access_tokens (id, user_id, application_id, token_hash, token_prefix, scopes, audience, expires_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8,$8)",
		accessID, userID, appID, tokenHashForTest(oldAccess), "gl_pat", []string{"openid", "profile"}, []string{"api"}, now.Add(time.Hour))
	exec("INSERT INTO guliangiam.refresh_tokens (id, user_id, application_id, token_hash, token_prefix, scopes, audience, access_token_id, family_id, expires_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$1,$9,$9,$9)",
		refreshID, userID, appID, tokenHashForTest(oldRefresh), "gl_prt", []string{"openid", "profile"}, []string{"api"}, accessID, now.Add(24*time.Hour))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM guliangiam.refresh_tokens WHERE id = $1", refreshID)
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM guliangiam.access_tokens WHERE id = $1", accessID)
	})

	uow := postgresuow.New(pool)
	tokensService := apptokens.NewService(clock.SystemClock{}, platformuuid.V7Generator{}, uow)
	svc := &Service{
		Clock:         clock.SystemClock{},
		UUIDGenerator: platformuuid.V7Generator{},
		UOW:           uow,
		RefreshTokens: pgRefreshExchanger{tokens: tokensService},
	}

	response, err := svc.ExchangeRefreshToken(tenantctx.System(ctx), ExchangeRefreshTokenCommand{
		RefreshToken: oldRefresh,
		ClientID:     clientID,
	})
	require.NoError(t, err)
	require.NotEmpty(t, response.AccessToken)
	require.NotEmpty(t, response.RefreshToken)
	require.Equal(t, "Bearer", response.TokenType)

	var revoked *time.Time
	require.NoError(t, pool.QueryRow(ctx, "SELECT revoked_at FROM guliangiam.refresh_tokens WHERE id = $1", refreshID).Scan(&revoked))
	require.NotNil(t, revoked, "old refresh token must be revoked after rotation")
}
