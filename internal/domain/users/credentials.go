package users

import (
	"fmt"
	"slices"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// PasswordAlgorithmArgon2ID is the only algorithm permitted for a locally
// stored password hash. New passwords (signup, reset, invite activation,
// self-service change, admin set, bootstrap seed) are always written with it.
const PasswordAlgorithmArgon2ID = "argon2id"

// AccessSecretGracePeriod 是访问密钥轮换的宽限期：轮换后的前 N 分钟内，
// 旧 access_secret 仍然可以通过校验（对 prev 哈希比对），给客户端滚动更新窗口。
const AccessSecretGracePeriod = 30 * time.Minute

// validPasswordAlgorithms is the write contract: any algorithm that is not
// listed here is rejected by Validate and must never be selected for a new
// password hash.
var validPasswordAlgorithms = []string{PasswordAlgorithmArgon2ID}

// legacyPasswordAlgorithms are read-only formats. They may already exist in
// user_credentials from historical imports or earlier releases; they are
// accepted only to verify an existing hash so the login path can immediately
// rewrite it as argon2id. They are never accepted by Validate and are never
// chosen for a new hash.
var legacyPasswordAlgorithms = []string{
	"bcrypt", "pbkdf2-salt", "sha256-salt", "sha512-salt",
}

type UserCredentials struct {
	UserID                string
	HasPassword           bool
	PasswordHash          string
	PasswordAlgo          string
	PasswordUpdatedAt     *time.Time
	AccessKey             string
	AccessSecretHash      string
	AccessSecretRotatedAt *time.Time
	NeedUpdatePassword    bool
	// AccessSecretHashPrev 是上一轮 access_secret 的哈希，轮换时将当前
	// AccessSecretHash 移入这里，为宽限期内的滚动校验留痕。
	AccessSecretHashPrev string
	// AccessSecretPrevExpiresAt 是 AccessSecretHashPrev 的过期时间戳；
	// 超过此时间后，旧 secret 自动失效，数据库层无需额外清理（SELECT 时
	// 根据此字段决定是否参与比对）。
	AccessSecretPrevExpiresAt *time.Time
}

// Validate checks the credential fields for business rule violations.
func (c *UserCredentials) Validate() error {
	if c.UserID == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if c.PasswordAlgo != "" && !slices.Contains(validPasswordAlgorithms, c.PasswordAlgo) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("unsupported password algorithm: %s", c.PasswordAlgo),
			map[string]any{"field": "algorithm", "value": c.PasswordAlgo, "allowed": append([]string{}, validPasswordAlgorithms...)},
		)
	}
	return nil
}

// PasswordVerificationAlgorithm returns the verifier required by the stored
// local password hash, and whether the row actually holds a local password.
//
// The hash, not the algorithm column, is the source of truth: a marker such as
// "ldap" on a credential row describes an external credential source and must
// never be mistaken for a hash format. An empty algorithm is the historical
// schema default and is read as argon2id, matching the hash that was written
// alongside it. Unsupported markers (ldap/plain/md5/unknown) return ok=false
// so every caller fails closed instead of feeding the value to a verifier.
func (c *UserCredentials) PasswordVerificationAlgorithm() (algorithm string, ok bool) {
	if c == nil || strings.TrimSpace(c.PasswordHash) == "" {
		return "", false
	}
	return localPasswordVerificationAlgorithm(c.PasswordAlgo)
}

// IsPasswordExpired returns true if the password was last updated at or beyond
// maxDays ago. It is the single max-age threshold implementation; app-layer
// decisions must call it instead of re-deriving the duration.
func (c *UserCredentials) IsPasswordExpired(now time.Time, maxDays int) bool {
	if c.PasswordUpdatedAt == nil || maxDays <= 0 {
		return false
	}
	maxDuration := time.Duration(maxDays) * 24 * time.Hour
	return now.Sub(*c.PasswordUpdatedAt) >= maxDuration
}

// HasAccessKey returns true if the user has an access key configured.
func (c *UserCredentials) HasAccessKey() bool {
	return c.AccessKey != "" && c.AccessSecretHash != ""
}

// AccessSecretVerifier verifies a plaintext secret against a stored hash.
// cred.CredManager satisfies this interface; the domain keeps its own minimal
// contract so it never imports the cred package.
type AccessSecretVerifier interface {
	IsPasswordCorrect(plainPwd, hashedPwd, salt string) bool
}

// VerifyAccessSecret checks a plaintext access secret against the current
// hash first, then — while the grace period is still open — against the
// previous hash. This is the single source of truth for rotation-aware
// secret verification: callers must use it instead of re-deriving the
// prev/expiry comparison, so a rotated client keeps working for
// AccessSecretGracePeriod after the switch and fails closed afterwards.
func (c *UserCredentials) VerifyAccessSecret(verifier AccessSecretVerifier, secret string, now time.Time) bool {
	if verifier == nil || strings.TrimSpace(secret) == "" {
		return false
	}
	if c.AccessSecretHash != "" && verifier.IsPasswordCorrect(secret, c.AccessSecretHash, "") {
		return true
	}
	if c.AccessSecretHashPrev != "" && c.AccessSecretPrevExpiresAt != nil && now.Before(*c.AccessSecretPrevExpiresAt) {
		return verifier.IsPasswordCorrect(secret, c.AccessSecretHashPrev, "")
	}
	return false
}

// localPasswordVerificationAlgorithm maps a stored algorithm to the verifier
// that can read its hash. Empty means the schema's historical default and is
// treated as argon2id.
func localPasswordVerificationAlgorithm(algorithm string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(algorithm))
	if normalized == "" || normalized == PasswordAlgorithmArgon2ID {
		return PasswordAlgorithmArgon2ID, true
	}
	if slices.Contains(legacyPasswordAlgorithms, normalized) {
		return normalized, true
	}
	return "", false
}
