package idv

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// newBizSeq returns a fresh 32-char hex merchant business sequence. It maps to
// Aliyun OuterOrderNo / MerchantBizId; the unique constraint on
// identity_verifications.biz_seq enforces idempotency and anti-replay (this
// deployment has no Redis, so the database is the single arbiter).
func newBizSeq() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", apperrors.WrapInternal(err, "idv: generate biz_seq")
	}
	return hex.EncodeToString(buf), nil
}

// subjectRef derives the de-identified subject reference: salted SM3 hex of
// the claimed ID number. The salt is domain-separated from the platform KEK so
// no new secret material is introduced; the plaintext ID number never leaves
// this function and must never be logged. Fail-closed when the KEK is absent.
func subjectRef(idNumber string) (string, error) {
	trimmed := strings.TrimSpace(idNumber)
	if trimmed == "" {
		return "", nil
	}
	key := config.EncryptionKeyCurrent()
	if key == "" {
		return "", apperrors.NewUnavailable("idv: subject reference derivation key is not configured")
	}
	salt := crypto.SM3Sum([]byte(key + "|guliangiam:idv:subject-ref:v1"))
	sum := crypto.SM3Sum(append(salt, []byte(trimmed)...))
	return hex.EncodeToString(sum), nil
}
