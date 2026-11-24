package crypto

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GM-G7 acceptance: v2 round-trips under both AES-256-GCM and SM4-GCM.
func TestAADEnvelopeV2_RoundTrip(t *testing.T) {
	ctx := AADContext{
		Table:   "guliangiam.providers",
		Column:  "client_secret",
		PKValue: "0190a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b",
	}

	t.Run("aes-256-gcm", func(t *testing.T) {
		encryptor := newStrictTestEncryptor(t)
		token, err := encryptor.EncryptColumn(ctx, "aes-row-secret")
		require.NoError(t, err)
		require.True(t, isV2EnvelopeToken(token), "v2 token must carry the -aad suffix: %q", token)
		assert.True(t, strings.HasPrefix(token, "v1-aes-aad:"))

		opened, err := encryptor.DecryptColumn(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, "aes-row-secret", opened)

		opened, err = encryptor.DecryptEnvelopeFieldWithAAD(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, "aes-row-secret", opened)
	})

	t.Run("sm4-gcm", func(t *testing.T) {
		key := make([]byte, 16)
		for i := range key {
			key[i] = byte(i + 1)
		}
		t.Setenv("ENCRYPTION_ALGORITHM", "sm4-gcm")
		t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(key))
		t.Setenv("ENCRYPTION_KEY", "")
		t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
		t.Setenv("ENCRYPTION_KEY_COMPAT", "")
		t.Setenv("IS_PRODUCTION", "false")
		encryptor := NewFieldEncryptor()

		token, err := encryptor.EncryptColumn(ctx, "sm4-row-secret")
		require.NoError(t, err)
		require.True(t, isV2EnvelopeToken(token), "v2 token must carry the -aad suffix: %q", token)
		assert.True(t, strings.HasPrefix(token, "v1-sm4-aad:"))

		opened, err := encryptor.DecryptColumn(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, "sm4-row-secret", opened)
	})
}

// GM-G7 acceptance 2: ciphertext moved to another row or column must fail.
func TestAADEnvelopeV2_CrossRowAndCrossColumnFail(t *testing.T) {
	encryptor := newStrictTestEncryptor(t)
	rowA := AADContext{Table: "guliangiam.providers", Column: "client_secret", PKValue: "row-a"}
	rowB := AADContext{Table: "guliangiam.providers", Column: "client_secret", PKValue: "row-b"}
	otherColumn := AADContext{Table: "guliangiam.providers", Column: "app_secret", PKValue: "row-a"}

	sealed, err := encryptor.EncryptColumn(rowA, "row-a-secret")
	require.NoError(t, err)

	_, err = encryptor.DecryptColumn(rowB, sealed)
	require.ErrorIs(t, err, ErrAADMismatch, "cross-row ciphertext must not open")

	_, err = encryptor.DecryptColumn(otherColumn, sealed)
	require.ErrorIs(t, err, ErrAADMismatch, "cross-column ciphertext must not open")

	_, err = encryptor.DecryptEnvelopeFieldWithAAD(rowB, sealed)
	require.ErrorIs(t, err, ErrAADMismatch)
}

// GM-G7 acceptance 3: v1 keeps reading, v0 only through DecryptColumn.
func TestAADEnvelopeV2_V1AndV0Compatibility(t *testing.T) {
	encryptor := newStrictTestEncryptor(t)
	ctx := AADContext{Table: "guliangiam.providers", Column: "client_secret", PKValue: "row-a"}

	v1Token, err := encryptor.EncryptField("v1-secret")
	require.NoError(t, err)
	require.False(t, isV2EnvelopeToken(v1Token))

	opened, err := encryptor.DecryptColumn(ctx, v1Token)
	require.NoError(t, err, "v1 tokens written before G7 must stay readable")
	assert.Equal(t, "v1-secret", opened)

	opened, err = encryptor.DecryptEnvelopeFieldWithAAD(ctx, v1Token)
	require.NoError(t, err)
	assert.Equal(t, "v1-secret", opened)

	v0Token := legacyDirectKEKToken(t, "v0-secret")
	opened, err = encryptor.DecryptColumn(ctx, v0Token)
	require.NoError(t, err, "v0 stays readable through the legacy-tolerant read")
	assert.Equal(t, "v0-secret", opened)

	_, err = encryptor.DecryptEnvelopeFieldWithAAD(ctx, v0Token)
	require.ErrorIs(t, err, ErrNotEnvelope, "the fail-closed read must reject v0")
}

// GM-G7 acceptance 6 (context side): v2 without a row binding never degrades.
func TestAADEnvelopeV2_RequiresContext(t *testing.T) {
	encryptor := newStrictTestEncryptor(t)
	ctx := AADContext{Table: "guliangiam.providers", Column: "client_secret", PKValue: "row-a"}

	sealed, err := encryptor.EncryptColumn(ctx, "bound-secret")
	require.NoError(t, err)

	_, err = encryptor.DecryptField(sealed)
	require.ErrorIs(t, err, ErrAADContext, "the no-AAD API must refuse v2 instead of guessing")

	_, err = encryptor.DecryptEnvelopeField(sealed)
	require.ErrorIs(t, err, ErrAADContext)

	_, err = encryptor.EncryptColumn(AADContext{Table: ctx.Table, Column: ctx.Column}, "unbound")
	require.ErrorIs(t, err, ErrAADContext, "a write without a primary key must fail, not fall back to v1")

	for _, pk := range []string{"row|a", "row\na", "row\ra", "row\x00a"} {
		_, err = encryptor.EncryptColumn(AADContext{Table: ctx.Table, Column: ctx.Column, PKValue: pk}, "ambiguous")
		require.ErrorIs(t, err, ErrAADContext, "reserved separator %q must be rejected", pk)
	}

	_, err = encryptor.EncryptColumn(AADContext{Table: "guliangiam;drop", Column: ctx.Column, PKValue: "row-a"}, "unsafe-table")
	require.ErrorIs(t, err, ErrAADContext)
}

func TestAADContext_Bytes(t *testing.T) {
	ctx := AADContext{
		Table:   "guliangiam.providers",
		Column:  "client_secret",
		PKValue: "0190A3B4-5C6D-7E8F-9A0B-1C2D3E4F5A6B",
	}
	payload, err := ctx.Bytes()
	require.NoError(t, err)
	assert.Equal(t,
		"v2|guliangiam.providers|client_secret|0190a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b",
		string(payload),
		"UUID primary keys must be normalized to lowercase hyphenated text")
}

// GM-G7 acceptance 6: a v2 authentication failure is never retried as v1/v0.
func TestAADEnvelopeV2_AuthFailureNoFallback(t *testing.T) {
	encryptor := newStrictTestEncryptor(t)
	ctx := AADContext{Table: "guliangiam.providers", Column: "client_secret", PKValue: "row-a"}

	sealed, err := encryptor.EncryptColumn(ctx, "tamper-target")
	require.NoError(t, err)

	parts := strings.SplitN(sealed, ":", 4)
	require.Len(t, parts, 4)
	ciphertext, err := base64.StdEncoding.DecodeString(parts[3])
	require.NoError(t, err)
	require.NotEmpty(t, ciphertext)
	ciphertext[len(ciphertext)-1] ^= 0x01
	parts[3] = base64.StdEncoding.EncodeToString(ciphertext)
	tampered := strings.Join(parts, ":")

	_, err = encryptor.DecryptColumn(ctx, tampered)
	require.ErrorIs(t, err, ErrAADMismatch, "tampering must be reported as an AAD/auth failure")
	require.False(t, errors.Is(err, ErrNotEnvelope), "a v2 token must never be reported as plaintext")

	_, err = encryptor.DecryptEnvelopeFieldWithAAD(ctx, tampered)
	require.ErrorIs(t, err, ErrAADMismatch)
}

// GM-G7 acceptance 4: RotateColumn upgrades v1->v2 and repeated runs are no-ops.
func TestRotateColumn_V1ToV2Idempotent(t *testing.T) {
	encryptor := newStrictTestEncryptor(t)
	ctx := AADContext{Table: "guliangiam.providers", Column: "client_secret", PKValue: "row-a"}

	v1Token, err := encryptor.EncryptField("rotate-me")
	require.NoError(t, err)

	rotated, changed, err := encryptor.RotateColumn(ctx, v1Token)
	require.NoError(t, err)
	require.True(t, changed)
	require.True(t, isV2EnvelopeToken(rotated), "rotation must produce v2")

	opened, err := encryptor.DecryptColumn(ctx, rotated)
	require.NoError(t, err)
	assert.Equal(t, "rotate-me", opened, "the rotated token must open with the same AAD")

	again, changed, err := encryptor.RotateColumn(ctx, rotated)
	require.NoError(t, err)
	assert.False(t, changed, "a second run must be a no-op")
	assert.Equal(t, rotated, again)

	otherRow := AADContext{Table: ctx.Table, Column: ctx.Column, PKValue: "row-b"}
	_, _, err = encryptor.RotateColumn(otherRow, rotated)
	require.ErrorIs(t, err, ErrAADMismatch, "rotation must not skip an unauthenticated current v2 token")
}
