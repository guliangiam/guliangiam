package pagination

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// cursorTTL 读取 CURSOR_TTL_SECONDS（秒）。空值或非法值回落默认一小时。
func cursorTTL() time.Duration {
	return time.Duration(config.CursorTTLSeconds()) * time.Second
}

type CursorToken struct {
	SortKey   string `json:"sk"`
	SortValue string `json:"sv"`
	ID        string `json:"id"`
	Timestamp int64  `json:"ts"`
}

func hmacKey() []byte {
	key := config.CursorHMACKey()
	if key == "" {
		key = "guliangiam-default-cursor-hmac-key"
	}
	return []byte(key)
}

func EncodeCursor(token CursorToken) (string, error) {
	payload, err := json.Marshal(token)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "marshal cursor")
	}

	mac := hmac.New(sha256.New, hmacKey())
	mac.Write(payload)
	sig := mac.Sum(nil)

	wrapped := struct {
		Payload   []byte `json:"p"`
		Signature []byte `json:"s"`
	}{
		Payload:   payload,
		Signature: sig,
	}

	wrappedJSON, err := json.Marshal(wrapped)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "marshal wrapped cursor")
	}

	return base64.URLEncoding.EncodeToString(wrappedJSON), nil
}

func DecodeCursor(encoded string) (*CursorToken, error) {
	wrappedJSON, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("invalid cursor encoding")
	}

	var wrapped struct {
		Payload   []byte `json:"p"`
		Signature []byte `json:"s"`
	}
	if err := json.Unmarshal(wrappedJSON, &wrapped); err != nil {
		return nil, apperrors.NewInvalidArgument("invalid cursor format")
	}

	mac := hmac.New(sha256.New, hmacKey())
	mac.Write(wrapped.Payload)
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(wrapped.Signature, expectedSig) {
		return nil, apperrors.NewInvalidArgument("cursor signature verification failed")
	}

	var token CursorToken
	if err := json.Unmarshal(wrapped.Payload, &token); err != nil {
		return nil, apperrors.NewInvalidArgument("invalid cursor payload")
	}

	created := time.Unix(token.Timestamp, 0)
	if time.Since(created) > cursorTTL() {
		return nil, apperrors.NewInvalidArgument("cursor expired")
	}

	return &token, nil
}
