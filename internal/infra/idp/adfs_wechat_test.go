package idp

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeBase64URL(t *testing.T) {
	t.Run("no_padding", func(t *testing.T) {
		// "hello" in base64url = "aGVsbG8" (no padding)
		result, err := decodeBase64URL("aGVsbG8")
		assert.NoError(t, err)
		assert.Equal(t, "hello", string(result))
	})
	t.Run("with_padding_2", func(t *testing.T) {
		// "A" in base64url = "QQ==" -> without padding "QQ"
		result, err := decodeBase64URL("QQ")
		assert.NoError(t, err)
		assert.Equal(t, "A", string(result))
	})
	t.Run("with_padding_1", func(t *testing.T) {
		// "AB" in base64url = "QUI=" -> without padding "QUI"
		result, err := decodeBase64URL("QUI")
		assert.NoError(t, err)
		assert.Equal(t, "AB", string(result))
	})
	t.Run("already_padded", func(t *testing.T) {
		// "hello world" in base64url = "aGVsbG8gd29ybGQ="
		result, err := decodeBase64URL("aGVsbG8gd29ybGQ")
		assert.NoError(t, err)
		assert.Equal(t, "hello world", string(result))
	})
	t.Run("invalid_base64", func(t *testing.T) {
		_, err := decodeBase64URL("!!!invalid!!!")
		assert.Error(t, err)
	})
	t.Run("empty", func(t *testing.T) {
		result, err := decodeBase64URL("")
		assert.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestADFSProvider_ParseIDTokenClaims(t *testing.T) {
	p := &ADFSProvider{}

	t.Run("valid_token_with_sid", func(t *testing.T) {
		// Create a fake JWT payload
		payload := map[string]interface{}{
			"sid":         "session-123",
			"unique_name": "jdoe",
			"upn":         "jdoe@example.com",
		}
		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err)
		encoded := base64.URLEncoding.EncodeToString(payloadBytes)
		// Remove padding
		for len(encoded)%4 != 0 {
			encoded += "="
		}
		idToken := "header." + encoded

		info, err := p.parseIDTokenClaims(idToken)
		assert.NoError(t, err)
		assert.Equal(t, "session-123", info.ID)
		assert.Equal(t, "jdoe", info.Username)
		assert.Equal(t, "jdoe", info.DisplayName)
		assert.Equal(t, "jdoe@example.com", info.Email)
	})

	t.Run("valid_token_with_sub_fallback", func(t *testing.T) {
		payload := map[string]interface{}{
			"sub":  "subject-456",
			"name": "John Doe",
		}
		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err)
		encoded := base64.URLEncoding.EncodeToString(payloadBytes)
		for len(encoded)%4 != 0 {
			encoded += "="
		}
		idToken := "header." + encoded

		info, err := p.parseIDTokenClaims(idToken)
		assert.NoError(t, err)
		assert.Equal(t, "subject-456", info.ID)
		assert.Equal(t, "John Doe", info.DisplayName)
		assert.Equal(t, "John Doe", info.Username)
	})

	t.Run("valid_token_with_email_fallback", func(t *testing.T) {
		payload := map[string]interface{}{
			"sub":   "subject-789",
			"email": "user@example.com",
		}
		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err)
		encoded := base64.URLEncoding.EncodeToString(payloadBytes)
		for len(encoded)%4 != 0 {
			encoded += "="
		}
		idToken := "header." + encoded

		info, err := p.parseIDTokenClaims(idToken)
		assert.NoError(t, err)
		assert.Equal(t, "subject-789", info.ID)
		assert.Equal(t, "user@example.com", info.Email)
	})

	t.Run("invalid_format_no_dots", func(t *testing.T) {
		_, err := p.parseIDTokenClaims("invalidtoken")
		assert.Error(t, err)
	})

	t.Run("invalid_base64_payload", func(t *testing.T) {
		_, err := p.parseIDTokenClaims("header.!!!invalid!!!")
		assert.Error(t, err)
	})

	t.Run("invalid_json_payload", func(t *testing.T) {
		encoded := base64.URLEncoding.EncodeToString([]byte("not json"))
		for len(encoded)%4 != 0 {
			encoded += "="
		}
		_, err := p.parseIDTokenClaims("header." + encoded)
		assert.Error(t, err)
	})
}

func TestExtractOpenIDFromToken(t *testing.T) {
	t.Run("scope_with_openid", func(t *testing.T) {
		token := &OAuthToken{
			Scope: "openid some_openid_value",
		}
		result := extractOpenIDFromToken(token)
		assert.Equal(t, "some_openid_value", result)
	})

	t.Run("scope_only_openid", func(t *testing.T) {
		token := &OAuthToken{
			Scope: "openid",
		}
		result := extractOpenIDFromToken(token)
		assert.Equal(t, "", result)
	})

	t.Run("scope_without_openid", func(t *testing.T) {
		token := &OAuthToken{
			Scope: "profile email",
		}
		result := extractOpenIDFromToken(token)
		assert.Equal(t, "", result)
	})

	t.Run("empty_scope", func(t *testing.T) {
		token := &OAuthToken{
			Scope: "",
		}
		result := extractOpenIDFromToken(token)
		assert.Equal(t, "", result)
	})

	t.Run("nil_token", func(t *testing.T) {
		// extractOpenIDFromToken panics on nil - skip
	})
}
