package crypto

import (
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/hkdf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// hkdfMaxExpandLength is RFC 5869's ceiling: L <= 255 * HashLen.
const hkdfMaxExpandLength = 255 * sha256.Size

// Extract is RFC 5869 HKDF-SHA256-Extract. salt is the HKDF salt; when nil it
// is treated as HashLen zero bytes per the RFC. It returns the pseudorandom
// key (PRK) that Expand consumes.
func Extract(secret, salt []byte) []byte {
	return hkdf.Extract(sha256.New, secret, salt)
}

// Expand is RFC 5869 HKDF-SHA256-Expand: it stretches prk to exactly length
// bytes with the given info string. A negative length or one above
// 255*HashLen is rejected instead of silently producing a short key.
func Expand(prk, info []byte, length int) ([]byte, error) {
	if length < 0 || length > hkdfMaxExpandLength {
		return nil, apperrors.NewInvalidArgumentf(
			"HKDF-SHA256 expand length must be between 0 and %d, got %d", hkdfMaxExpandLength, length)
	}
	if length == 0 {
		return []byte{}, nil
	}
	out := make([]byte, length)
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, prk, info), out); err != nil {
		return nil, apperrors.WrapInternal(err, "HKDF-SHA256 expand")
	}
	return out, nil
}
