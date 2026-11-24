package crypto

import (
	"encoding/base64"
	"log/slog"
	"strings"
	"sync"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var (
	globalEncryptor   *FieldEncryptor
	globalEncryptorMu sync.Mutex
)

// FieldEncryptor provides envelope-encrypted field-level encryption with key
// versioning and rotation support.
//
// It is a thin wrapper over EnvelopeEncryptor that resolves KEKs from
// environment variables and exposes the historical EncryptField/DecryptField/
// RotateEncryptedData API used by repositories.
//
// Algorithm selection (ADR-031 Decision 4):
//   - ENCRYPTION_ALGORITHM  — 可选，默认 aes-256-gcm；可选值：sm4-gcm / gmsm
//   - SM4 是显式能力位：若 ENCRYPTION_ALGORITHM=sm4-gcm 但 suite.go 的 SM4
//     能力探测（probeSM4）失败，则 panic 拒绝启动（不做静默降级）
//
// Key resolution:
//   - ENCRYPTION_KEY_CURRENT  -> KEK version 1 (active, used for new encryptions)
//   - ENCRYPTION_KEY          -> legacy alias for version 1
//   - ENCRYPTION_KEY_PREVIOUS -> KEK version 0 (decryption fallback only)
//   - ENCRYPTION_KEY_COMPAT   -> 另一算法的解密密钥（仅算法切换重叠窗口使用）
//
// A missing current key is a hard failure (DB-058): confidential fields must
// never fall back to plaintext storage.
type FieldEncryptor struct {
	envelope *EnvelopeEncryptor
}

// NewFieldEncryptor creates a FieldEncryptor backed by an EnvelopeEncryptor
// whose KEKs are resolved from the configured KeyProvider (environment variables
// by default, Vault when KMS_PROVIDER=vault).
//
// It panics on invalid configuration and is kept for tests and non-startup
// paths. Bootstrap uses NewFieldEncryptorChecked instead so invalid key material
// aborts startup before any listener exists, rather than on the first
// confidential read or write.
//
// Version assignment:
//   - ENCRYPTION_KEY_CURRENT set -> current version = 1, key maps to v1
//   - ENCRYPTION_KEY only (legacy) -> current version = 0, key maps to v0
//   - ENCRYPTION_KEY_PREVIOUS -> always maps to v0 (decryption fallback)
//
// This ensures that when rotating from ENCRYPTION_KEY to ENCRYPTION_KEY_CURRENT,
// the old key lands at v0 and old v0 tokens decrypt correctly.
func NewFieldEncryptor() *FieldEncryptor {
	encryptor, err := NewFieldEncryptorChecked()
	if err != nil {
		panic(err.Error())
	}
	return encryptor
}

// NewFieldEncryptorChecked constructs and validates a FieldEncryptor from the
// configured KeyProvider. It is the single rule set for key material: base64
// decoding, decoded length, algorithm selection and SM4 availability.
//
// The returned error names the offending environment variable and the reason;
// it never includes key material.
func NewFieldEncryptorChecked() (*FieldEncryptor, error) {
	// ADR-031 Decision 4: 算法选择从环境变量读，默认 AES-256-GCM（向后兼容）
	alg, err := resolveAlgorithmEnv()
	if err != nil {
		return nil, err
	}
	expectedKeySize, err := algorithmKeySize(alg)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "internal: unknown field encryption algorithm")
	}

	currentKey := config.EncryptionKeyCurrent()
	legacyKey := config.EncryptionKeyLegacy()

	if currentKey == "" && legacyKey == "" {
		return nil, apperrors.NewInvalidArgumentf("ENCRYPTION_KEY_CURRENT (or ENCRYPTION_KEY) is required: confidential fields must never fall back to plaintext storage; set a %d-byte base64-encoded key for %s (development and CI must use a dedicated test key)", expectedKeySize, algName(alg))
	}

	keks := map[int][]byte{}
	currentVersion := 0

	if currentKey != "" {
		currentBytes, err := base64.StdEncoding.DecodeString(currentKey)
		if err != nil {
			return nil, apperrors.Wrapf(apperrors.CodeInvalidArgument, err, "ENCRYPTION_KEY_CURRENT is not valid base64")
		}
		if len(currentBytes) != expectedKeySize {
			return nil, apperrors.NewInvalidArgumentf("ENCRYPTION_KEY_CURRENT must decode to exactly %d bytes for %s, got %d bytes", expectedKeySize, algName(alg), len(currentBytes))
		}
		keks[1] = currentBytes
		currentVersion = 1
	} else {
		// Legacy single-key mode: ENCRYPTION_KEY is the current key at v0
		legacyBytes, err := base64.StdEncoding.DecodeString(legacyKey)
		if err != nil {
			return nil, apperrors.Wrapf(apperrors.CodeInvalidArgument, err, "ENCRYPTION_KEY is not valid base64")
		}
		if len(legacyBytes) != expectedKeySize {
			return nil, apperrors.NewInvalidArgumentf("ENCRYPTION_KEY must decode to exactly %d bytes for %s, got %d bytes", expectedKeySize, algName(alg), len(legacyBytes))
		}
		keks[0] = legacyBytes
		currentVersion = 0
	}

	prevKey := config.EncryptionKeyPrevious()
	if prevKey != "" {
		prevBytes, err := base64.StdEncoding.DecodeString(prevKey)
		if err != nil {
			return nil, apperrors.Wrapf(apperrors.CodeInvalidArgument, err, "ENCRYPTION_KEY_PREVIOUS is not valid base64")
		}
		if len(prevBytes) != expectedKeySize {
			return nil, apperrors.NewInvalidArgumentf("ENCRYPTION_KEY_PREVIOUS must decode to exactly %d bytes for %s, got %d bytes", expectedKeySize, algName(alg), len(prevBytes))
		}
		// Only set v0 if not already set by legacy mode
		if _, exists := keks[0]; !exists {
			keks[0] = prevBytes
		}
		slog.Info("encryption key rotation: previous key (v0) loaded for decryption fallback")
	}

	keksByAlg := map[Algorithm]map[int][]byte{alg: keks}

	// ENCRYPTION_KEY_COMPAT 是算法切换重叠窗口的解密专用密钥：算法从
	// aes-256-gcm 切到 sm4-gcm（或反向）时，用它解开另一算法写下的旧 token。
	// 长度由另一算法决定（AES-256=32B，SM4=16B），映射到与当前密钥相同的版本。
	if compatKey := config.EncryptionKeyCompat(); compatKey != "" {
		otherAlg := AlgorithmAES256GCM
		if alg == AlgorithmAES256GCM {
			otherAlg = AlgorithmSM4GCM
		}
		expectedOtherSize, err := algorithmKeySize(otherAlg)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "internal: unknown compatibility algorithm")
		}
		compatBytes, err := base64.StdEncoding.DecodeString(compatKey)
		if err != nil {
			return nil, apperrors.Wrapf(apperrors.CodeInvalidArgument, err, "ENCRYPTION_KEY_COMPAT is not valid base64")
		}
		if len(compatBytes) != expectedOtherSize {
			return nil, apperrors.NewInvalidArgumentf("ENCRYPTION_KEY_COMPAT must decode to exactly %d bytes for %s, got %d bytes",
				expectedOtherSize, algName(otherAlg), len(compatBytes))
		}
		keksByAlg[otherAlg] = map[int][]byte{currentVersion: compatBytes}
	}

	env, err := NewEnvelopeEncryptorMultiAlgorithm(keksByAlg, currentVersion, alg)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "failed to create envelope encryptor (%s)", algName(alg))
	}

	return &FieldEncryptor{envelope: env}, nil
}

// isAESAlgorithm reports whether the configured literal selects AES-256-GCM.
// It recognizes the aliases accepted by resolveAlgorithmEnv.
func isAESAlgorithm(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "aes", "aes256", "aes-256-gcm":
		return true
	default:
		return false
	}
}

// resolveAlgorithmEnv 从 ENCRYPTION_ALGORITHM 环境变量解析算法选择。
//
//   - CRYPTO_PROFILE=gm 且 ENCRYPTION_ALGORITHM 未设置：选择 SM4-GCM（gm 档的
//     字段加密默认算法），仍走 SM4 能力探测；
//   - CRYPTO_PROFILE=gm 且显式 AES：返回错误拒绝启动（startup 的
//     ValidateRuntimeProfile 会先返回结构化错误，这里是纵深防御）；
//   - CRYPTO_PROFILE=default/空：未设置时保持 AES-256-GCM 的既有行为。
//
// 若显式请求 SM4 但能力不可用则返回错误（禁止静默降级）。
func resolveAlgorithmEnv() (Algorithm, error) {
	v := config.EncryptionAlgorithm()
	gmProfile := strings.EqualFold(config.CryptoProfile(), string(ProfileGM))
	if v == "" {
		if !gmProfile {
			return AlgorithmAES256GCM, nil
		}
		v = "sm4-gcm"
	}
	if gmProfile && isAESAlgorithm(v) {
		return 0, apperrors.NewInvalidArgumentf("CRYPTO_PROFILE=gm forbids ENCRYPTION_ALGORITHM=%q: field encryption must use sm4-gcm (no silent downgrade to AES)", v)
	}
	switch v {
	case "", "aes", "aes256", "aes-256-gcm":
		return AlgorithmAES256GCM, nil
	case "sm4", "sm4-gcm", "gmsm":
		// ADR-031 Decision 4: 能力缺失时拒绝启用而非静默降级
		if !sm4Available() {
			return 0, apperrors.NewUnavailable("ENCRYPTION_ALGORITHM=SM4 但 github.com/emmansun/gmsm/sm4 不可用；国密是显式能力位，缺失时拒绝启用而非静默降级")
		}
		return AlgorithmSM4GCM, nil
	default:
		return 0, apperrors.NewInvalidArgumentf("unknown ENCRYPTION_ALGORITHM=%q — supported: aes-256-gcm, sm4-gcm", v)
	}
}

// sm4Available 由 suite.go 的单一 SM4 能力探测（probeSM4）实现；
// 本文件只消费判定结果，不再保留第二套探测口径。

// InitFieldEncryptor constructs and publishes the process-wide FieldEncryptor.
// Bootstrap calls it once before creating any listener, so invalid key material
// (base64, decoded length, algorithm, SM4 availability) aborts startup with a
// returned error instead of a panic on the first confidential read or write.
//
// The published instance is what GlobalFieldEncryptor returns afterwards, so the
// startup construction is not repeated lazily at runtime.
func InitFieldEncryptor() (*FieldEncryptor, error) {
	encryptor, err := NewFieldEncryptorChecked()
	if err != nil {
		return nil, err
	}
	globalEncryptorMu.Lock()
	globalEncryptor = encryptor
	globalEncryptorMu.Unlock()
	return encryptor, nil
}

// GlobalFieldEncryptor returns the process-wide encryptor. Outside the startup
// path (tests, command-line tools) it lazily constructs one and keeps the
// historical panic-on-invalid-configuration contract. After InitFieldEncryptor
// has published an instance, every caller reuses it.
func GlobalFieldEncryptor() *FieldEncryptor {
	globalEncryptorMu.Lock()
	defer globalEncryptorMu.Unlock()
	if globalEncryptor == nil {
		globalEncryptor = NewFieldEncryptor()
	}
	return globalEncryptor
}

// FieldEncryptionConfigured reports whether a KEK is configured. Operator
// commands use it to fail before touching data instead of triggering the
// process-start panic in NewFieldEncryptor.
func FieldEncryptionConfigured() bool {
	return config.EncryptionKeyCurrent() != "" || config.EncryptionKeyLegacy() != ""
}

// EncryptField encrypts plaintext using the v1 envelope scheme with the current
// KEK version. It is reserved for non-column payloads (sealed JSON configs,
// logout-token caches) that have no row identity; encrypted columns must use
// EncryptColumn so new writes are v2 and carry the ADR-015 AAD.
func (e *FieldEncryptor) EncryptField(plaintext string) (string, error) {
	return e.envelope.Encrypt(plaintext)
}

// DecryptField decrypts a versioned envelope token. Legacy (non-envelope)
// tokens produced by the previous direct-KEK implementation are decrypted
// transparently using available KEKs. A v2 token cannot be opened here because
// this API has no row binding; it fails with ErrAADContext.
func (e *FieldEncryptor) DecryptField(ciphertext string) (string, error) {
	return e.envelope.Decrypt(ciphertext)
}

// DecryptEnvelopeField decrypts a versioned v1 or v2 envelope token and refuses
// anything else. Unlike DecryptField it has no legacy fallback and no plaintext
// passthrough: provider credentials use it so a row that predates the encryption
// backfill fails closed instead of being used as plaintext. v2 tokens require
// the column-bound variant.
func (e *FieldEncryptor) DecryptEnvelopeField(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	if !IsEnvelopeToken(token) {
		return "", ErrNotEnvelope
	}
	return e.envelope.decryptEnvelope(token, nil)
}

// EncryptColumn seals a column value into a v2 envelope bound to the row AAD.
// A missing or ambiguous AAD context is an error, never a downgrade to v1.
// An empty plaintext stays empty ("not configured") and writes no ciphertext.
func (e *FieldEncryptor) EncryptColumn(ctx AADContext, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	aad, err := ctx.Bytes()
	if err != nil {
		return "", err
	}
	return e.envelope.EncryptWithAAD(plaintext, aad)
}

// DecryptColumn decrypts v2 (with the row AAD), v1 and v0 values. This is the
// column-read equivalent of DecryptField: v2 authentication failures are
// returned as ErrAADMismatch and never retried as v1 or v0.
func (e *FieldEncryptor) DecryptColumn(ctx AADContext, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	aad, err := ctx.Bytes()
	if err != nil {
		return "", err
	}
	return e.envelope.DecryptWithAAD(ciphertext, aad)
}

// DecryptEnvelopeFieldWithAAD is the fail-closed column read: it accepts v2
// (authenticated with the row AAD) and v1, and rejects v0/plaintext with
// ErrNotEnvelope. It never falls back after a v2 authentication failure.
func (e *FieldEncryptor) DecryptEnvelopeFieldWithAAD(ctx AADContext, token string) (string, error) {
	if token == "" {
		return "", nil
	}
	if !IsEnvelopeToken(token) {
		return "", ErrNotEnvelope
	}
	aad, err := ctx.Bytes()
	if err != nil {
		return "", err
	}
	return e.envelope.decryptEnvelope(token, aad)
}

// RotateColumn rewrites a column value into the v2 generation. v1/v0 values are
// upgraded, a v2 value bound to the same AAD at the current KEK/algorithm is
// authenticated and returned unchanged, so repeated runs are idempotent.
func (e *FieldEncryptor) RotateColumn(ctx AADContext, ciphertext string) (rotated string, changed bool, err error) {
	if ciphertext == "" {
		return "", false, nil
	}
	aad, err := ctx.Bytes()
	if err != nil {
		return "", false, err
	}
	return e.envelope.ReencryptWithAAD(ciphertext, aad)
}

// RotateEncryptedData re-encrypts a non-column token so it uses the current
// KEK version. It keeps the v1 generation (no AAD) and refuses v2 tokens
// because it has no row binding; column rotation must use RotateColumn.
func (e *FieldEncryptor) RotateEncryptedData(ciphertext string) (rotated string, changed bool, err error) {
	return e.envelope.Reencrypt(ciphertext)
}

// HasPreviousKey returns true if a previous (v0) KEK is configured,
// indicating that key rotation may be in progress.
func (e *FieldEncryptor) HasPreviousKey() bool {
	return e.envelope.HasVersion(0)
}

// IsEnabled reports whether field encryption is active. Encryption is never
// disabled: a missing key aborts startup (DB-058).
func (e *FieldEncryptor) IsEnabled() bool {
	return true
}

// CurrentKeyVersion returns the active KEK version used for new encryptions.
func (e *FieldEncryptor) CurrentKeyVersion() int {
	return e.envelope.CurrentVersion()
}

// Algorithm returns the symmetric cipher in use (ADR-031 Decision 4 observability).
func (e *FieldEncryptor) Algorithm() Algorithm {
	return e.envelope.Algorithm()
}
