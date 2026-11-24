package users

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

const testTotpSecret = "JBSWY3DPEHPK3PXP" // #nosec G101 -- deterministic fixture

func TestTotpSecret_SealAndOpen(t *testing.T) {
	sealed, err := sealTotpSecret("user-1", testTotpSecret)
	require.NoError(t, err)
	require.True(t, crypto.IsEnvelopeToken(sealed))

	user, err := toUser(userRow{ID: "user-1", TotpSecret: sealed})
	require.NoError(t, err)
	assert.Equal(t, testTotpSecret, user.TotpSecret)

	// Empty ("MFA not configured") must not become an envelope token.
	empty, err := sealTotpSecret("user-1", "")
	require.NoError(t, err)
	assert.Empty(t, empty)

	// A pre-GM-G2 plaintext row fails closed instead of being used as a seed.
	plain, err := openTotpSecret("user-1", testTotpSecret)
	require.Error(t, err)
	assert.Empty(t, plain)
}

func TestUpsertUserSubtables_SealsTotpSecret(t *testing.T) {
	var mfaArgs []any
	mockDB := &shared.MockDBTX{ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		if sql == queryUpsertUserMfaConfig {
			mfaArgs = args
		}
		return pgconn.NewCommandTag("INSERT 1"), nil
	}}
	emptyJSON := []byte("[]")
	require.NoError(t, NewRepository(mockDB).upsertUserSubtables(context.Background(), mockDB,
		&domain.User{ID: "user-1", TotpSecret: testTotpSecret},
		nil, emptyJSON, emptyJSON, emptyJSON, emptyJSON, emptyJSON, emptyJSON, emptyJSON))

	require.Len(t, mfaArgs, 10)
	stored, ok := mfaArgs[3].(string)
	require.True(t, ok, "totp_secret argument must be a string, got %T", mfaArgs[3])
	require.True(t, crypto.IsEnvelopeToken(stored), "upsert must bind ciphertext")
	opened, err := openTotpSecret("user-1", stored)
	require.NoError(t, err)
	assert.Equal(t, testTotpSecret, opened)
}
