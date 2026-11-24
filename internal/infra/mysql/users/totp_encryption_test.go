package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

const testTotpSecret = "JBSWY3DPEHPK3PXP" // #nosec G101 -- deterministic fixture

func TestTotpSecret_SealAndOpen(t *testing.T) {
	sealed, err := sealTotpSecret("user-1", testTotpSecret)
	require.NoError(t, err)
	require.True(t, crypto.IsEnvelopeToken(sealed))

	opened, err := openTotpSecret("user-1", sealed)
	require.NoError(t, err)
	assert.Equal(t, testTotpSecret, opened)

	row := userRow{ID: "user-1", TotpSecret: sealed,
		RecoveryCodeHashes: []byte("[]"), Addresses: []byte("[]"), ManagedAccounts: []byte("[]"), FaceIds: []byte("[]"),
		Cart: []byte("[]"), MfaItems: []byte("[]"), MfaAccounts: []byte("[]"), ApplicationScopes: []byte("[]")}
	user, err := toUser(row)
	require.NoError(t, err)
	assert.Equal(t, testTotpSecret, user.TotpSecret)

	empty, err := sealTotpSecret("user-1", "")
	require.NoError(t, err)
	assert.Empty(t, empty)

	plain, err := openTotpSecret("user-1", testTotpSecret)
	require.Error(t, err)
	assert.Empty(t, plain)
}
