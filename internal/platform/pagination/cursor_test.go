package pagination

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeCursor_RoundTrip(t *testing.T) {
	token := CursorToken{
		SortKey:   "created_at",
		SortValue: "2026-01-01T00:00:00Z",
		ID:        "item-001",
		Timestamp: time.Now().Unix(),
	}

	encoded, err := EncodeCursor(token)
	assert.NoError(t, err, "encoding should succeed")
	assert.NotEmpty(t, encoded, "encoded cursor should not be empty")

	decoded, err := DecodeCursor(encoded)
	assert.NoError(t, err, "decoding should succeed")
	assert.Equal(t, token.SortKey, decoded.SortKey)
	assert.Equal(t, token.SortValue, decoded.SortValue)
	assert.Equal(t, token.ID, decoded.ID)
	assert.Equal(t, token.Timestamp, decoded.Timestamp)
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, err := DecodeCursor("!!!invalid-base64!!!")
	assert.Error(t, err, "should reject invalid base64")
}

func TestDecodeCursor_InvalidJSON(t *testing.T) {
	// Valid base64 but invalid JSON structure
	encoded := "aW52YWxpZCBqc29u"
	_, err := DecodeCursor(encoded)
	assert.Error(t, err, "should reject invalid JSON")
}

func TestDecodeCursor_TamperedSignature(t *testing.T) {
	token := CursorToken{
		SortKey:   "created_at",
		SortValue: "2026-01-01T00:00:00Z",
		ID:        "item-001",
		Timestamp: time.Now().Unix(),
	}

	encoded, err := EncodeCursor(token)
	assert.NoError(t, err)

	// Tamper with the encoded string by changing last character
	tampered := encoded[:len(encoded)-1] + "X"
	_, err = DecodeCursor(tampered)
	assert.Error(t, err, "should reject tampered cursor")
}

func TestDecodeCursor_ExpiredCursor(t *testing.T) {
	t.Setenv("CURSOR_TTL_SECONDS", "")
	token := CursorToken{
		SortKey:   "created_at",
		SortValue: "2020-01-01T00:00:00Z",
		ID:        "item-old",
		Timestamp: time.Now().Add(-2 * time.Hour).Unix(), // expired
	}

	encoded, err := EncodeCursor(token)
	assert.NoError(t, err)

	_, err = DecodeCursor(encoded)
	assert.Error(t, err, "should reject expired cursor")
}

func TestDecodeCursor_RespectsCursorTTLSeconds(t *testing.T) {
	// A two-second-old cursor is rejected under a one-second TTL...
	t.Setenv("CURSOR_TTL_SECONDS", "1")
	stale := CursorToken{
		SortKey:   "created_at",
		SortValue: "2026-01-01T00:00:00Z",
		ID:        "item-ttl",
		Timestamp: time.Now().Add(-2 * time.Second).Unix(),
	}
	encoded, err := EncodeCursor(stale)
	assert.NoError(t, err)
	_, err = DecodeCursor(encoded)
	assert.Error(t, err, "should reject a cursor older than CURSOR_TTL_SECONDS")

	// ...but accepted again once the TTL is widened.
	t.Setenv("CURSOR_TTL_SECONDS", "3600")
	fresh := CursorToken{
		SortKey:   "created_at",
		SortValue: "2026-01-01T00:00:00Z",
		ID:        "item-ttl",
		Timestamp: time.Now().Add(-2 * time.Second).Unix(),
	}
	encoded, err = EncodeCursor(fresh)
	assert.NoError(t, err)
	_, err = DecodeCursor(encoded)
	assert.NoError(t, err, "should accept a cursor within CURSOR_TTL_SECONDS")
}

func TestCursorToken_Fields(t *testing.T) {
	token := CursorToken{
		SortKey:   "name",
		SortValue: "alice",
		ID:        "user-001",
		Timestamp: time.Now().Unix(),
	}

	assert.Equal(t, "name", token.SortKey)
	assert.Equal(t, "alice", token.SortValue)
	assert.Equal(t, "user-001", token.ID)
}
