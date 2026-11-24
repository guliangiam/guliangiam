package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/emmansun/gmsm/sm4"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// envelopeV2TokenPattern matches the ADR-015 v2 shape
// (v{KEK}-{alg}-aad:base64:base64:base64). "-aad" is the only discriminator;
// the three payload segments are identical to v1.
var envelopeV2TokenPattern = regexp.MustCompile(`^v\d+-[a-z0-9]+-aad:[A-Za-z0-9+/=]+:[A-Za-z0-9+/=]+:[A-Za-z0-9+/=]+$`)

// envelopeV1TokenPattern matches the existing envelope shape
// (v{KEK}[-{alg}]:base64:base64:base64). The optional algorithm suffix
// (aes/sm4) lets a reader select the cipher from the token itself during an
// ENCRYPTION_ALGORITHM switch; tokens written before the suffix existed remain
// valid and fall back to the configured algorithm. This pattern cannot match a
// v2 token because the optional tag has no second hyphen.
var envelopeV1TokenPattern = regexp.MustCompile(`^v\d+(-[a-z0-9]+)?:[A-Za-z0-9+/=]+:[A-Za-z0-9+/=]+:[A-Za-z0-9+/=]+$`)

// ErrNotEnvelope reports a stored value that is not a versioned envelope token.
// Callers enforcing an encrypted-at-rest invariant must treat it as a data
// error, never as plaintext to be used.
var ErrNotEnvelope = errors.New("crypto: value is not an envelope-encrypted token")

// IsEnvelopeToken reports whether token structurally matches a v1 or v2
// envelope. It does not decrypt or authenticate the payload; v2 tokens still
// require the row AAD context before they can be opened.
func IsEnvelopeToken(token string) bool {
	if isV2EnvelopeToken(token) {
		return true
	}
	if !envelopeV1TokenPattern.MatchString(token) {
		return false
	}
	// "v{N}-aad:" would match the optional v1 algorithm slot; it is not a valid
	// v1 header because ADR-015 requires an algorithm before the AAD suffix.
	headerPart, _, _ := strings.Cut(token, ":")
	return !strings.HasSuffix(headerPart, "-aad")
}

func isV2EnvelopeToken(token string) bool {
	return envelopeV2TokenPattern.MatchString(token)
}

// Algorithm identifies the symmetric cipher used for both KEK wrapping
// and DEK encryption in the envelope scheme. ADR-031 Decision 4 prescribes
// SM4 as an optional capability alongside the default AES-256.
//
// New tokens carry the algorithm as a header suffix (vN-aes / vN-sm4) so a
// reader can decrypt across an ENCRYPTION_ALGORITHM switch. Untagged legacy
// tokens still decrypt with the configured algorithm.
type Algorithm int

const (
	AlgorithmAES256GCM Algorithm = iota // default — AES-256-GCM (ADR-009)
	AlgorithmSM4GCM                     // 国密 SM4-GCM (GB/T 32907, ADR-031)
)

// algorithmKeySize returns the required KEK/DEK length for the given algorithm.
func algorithmKeySize(a Algorithm) (int, error) {
	switch a {
	case AlgorithmAES256GCM:
		return 32, nil
	case AlgorithmSM4GCM:
		return 16, nil
	default:
		return 0, apperrors.NewInvalidArgumentf("unknown encryption algorithm: %d", a)
	}
}

// algName returns a human-readable name for the algorithm, used in
// panic messages, error wrapping and logs.
func algName(a Algorithm) string {
	switch a {
	case AlgorithmAES256GCM:
		return "AES-256-GCM"
	case AlgorithmSM4GCM:
		return "SM4-GCM"
	default:
		return "unknown"
	}
}

// algorithmTag / algorithmFromTag 把算法映射为 token 头部的短标识。
// 标识只用于选择 cipher，不参与任何密钥派生。
func algorithmTag(a Algorithm) (string, bool) {
	switch a {
	case AlgorithmAES256GCM:
		return "aes", true
	case AlgorithmSM4GCM:
		return "sm4", true
	default:
		return "", false
	}
}

func algorithmFromTag(tag string) (Algorithm, bool) {
	switch tag {
	case "aes":
		return AlgorithmAES256GCM, true
	case "sm4":
		return AlgorithmSM4GCM, true
	default:
		return 0, false
	}
}

// envelopeHeader 是 token 前缀 v{N}[-{alg}][-aad] 的解析结果。
type envelopeHeader struct {
	version   int
	algorithm Algorithm
	tagged    bool
	aad       bool
}

// parseEnvelopeHeader enforces the ADR-015 discriminator order: the "-aad"
// suffix is only valid after a concrete algorithm tag, so a v2 header can never
// be mistaken for a v1 KEK/algorithm prefix.
func parseEnvelopeHeader(value string) (envelopeHeader, error) {
	if !strings.HasPrefix(value, "v") {
		return envelopeHeader{}, apperrors.NewInvalidArgument("envelope token version must start with 'v'")
	}
	fields := strings.Split(strings.TrimPrefix(value, "v"), "-")
	if len(fields) > 3 {
		return envelopeHeader{}, apperrors.NewInvalidArgumentf("invalid envelope header: %s", value)
	}
	version, err := strconv.Atoi(fields[0])
	if err != nil {
		return envelopeHeader{}, apperrors.NewInvalidArgumentf("invalid envelope version: %s", value)
	}
	header := envelopeHeader{version: version}
	if len(fields) == 1 {
		return header, nil
	}
	if fields[1] == "aad" {
		return envelopeHeader{}, apperrors.NewInvalidArgumentf("envelope AAD suffix requires an algorithm tag: %s", value)
	}
	algorithm, ok := algorithmFromTag(fields[1])
	if !ok {
		return envelopeHeader{}, apperrors.NewInvalidArgumentf("unknown envelope algorithm tag %q", fields[1])
	}
	header.algorithm, header.tagged = algorithm, true
	if len(fields) == 2 {
		return header, nil
	}
	if fields[2] != "aad" {
		return envelopeHeader{}, apperrors.NewInvalidArgumentf("unknown envelope suffix %q", fields[2])
	}
	header.aad = true
	return header, nil
}

// newCipher creates a cipher.Block for the given algorithm and key.
// AES 走标准库 crypto/aes；SM4 走唯一国密实现 github.com/emmansun/gmsm/sm4。
func newCipher(alg Algorithm, key []byte) (cipher.Block, error) {
	switch alg {
	case AlgorithmAES256GCM:
		return aes.NewCipher(key)
	case AlgorithmSM4GCM:
		return sm4.NewCipher(key)
	default:
		return nil, apperrors.NewInvalidArgumentf("unknown algorithm: %d", alg)
	}
}

// EnvelopeEncryptor implements envelope encryption with key versioning.
//
// Each encryption operation generates a fresh Data Encryption Key (DEK),
// encrypts the plaintext with the DEK (algorithm-selected GCM), and wraps
// the DEK with the active Key Encryption Key (KEK). The stored token
// carries the KEK version so decryption can select the correct KEK to
// unwrap the DEK.
//
// Token format (versioned envelope):
//
//	v{N}[-{alg}]:base64(encrypted_dek):base64(nonce):base64(ciphertext)   (v1)
//	v{N}-{alg}-aad:base64(encrypted_dek):base64(nonce):base64(ciphertext) (v2)
//
// - N           : KEK version (the current key is always the highest version)
// - encrypted_dek: DEK wrapped with KEK version N (GCM, nonce prepended)
// - nonce        : GCM nonce used for DEK -> plaintext encryption
// - ciphertext   : plaintext encrypted with the DEK
//
// v2 additionally binds both GCM layers to the ADR-015 row AAD
// ("v2|{table}|{column}|{primary_key_value}"); the AAD is never stored in the
// token. v1 remains readable for non-column payloads and pre-G7 rows.
//
// Backward compatibility: tokens matching neither envelope shape are treated as
// legacy v0 tokens encrypted directly with the KEK (the previous FieldEncryptor
// format). Only that branch is a legacy fallback; authentication failures never
// downgrade a token.
//
// Multi-algorithm mode holds one KEK set per algorithm so an old AES token can
// still be decrypted after ENCRYPTION_ALGORITHM switches to sm4-gcm. Writes
// always use `algorithm`; reads select the algorithm from the token header.
type EnvelopeEncryptor struct {
	keksByAlg map[Algorithm]map[int]cipher.AEAD // algorithm -> version -> KEK GCM
	current   int                               // active KEK version used for new encryptions
	dekSize   int                               // DEK size in bytes (32 for AES-256, 16 for SM4)
	algorithm Algorithm                         // ADR-031 Decision 4 — 选择的对称算法
}

// NewEnvelopeEncryptor builds an EnvelopeEncryptor from the provided versioned
// KEK map. The current version must be present in the map.
//
// 默认使用 AES-256-GCM（向后兼容 ADR-009 行为）。如需使用 SM4-GCM 国密算法，
// 请使用 NewEnvelopeEncryptorWithAlgorithm 显式指定 AlgorithmSM4GCM。
func NewEnvelopeEncryptor(keks map[int][]byte, current int) (*EnvelopeEncryptor, error) {
	return NewEnvelopeEncryptorWithAlgorithm(keks, current, AlgorithmAES256GCM)
}

// NewEnvelopeEncryptorWithAlgorithm 是 ADR-031 Decision 4 新增的算法选择入口。
// keks 里每个 key 长度必须匹配 algorithm（32 for AES-256, 16 for SM4）。
func NewEnvelopeEncryptorWithAlgorithm(keks map[int][]byte, current int, alg Algorithm) (*EnvelopeEncryptor, error) {
	return NewEnvelopeEncryptorMultiAlgorithm(map[Algorithm]map[int][]byte{alg: keks}, current, alg)
}

// NewEnvelopeEncryptorMultiAlgorithm 允许同一进程在算法切换重叠窗口内同时
// 持有新旧两套 KEK：写入使用 currentAlg，读取按 token 头部算法标识选择。
// 非当前算法的 KEK 集仅用于解密，可以为空。
func NewEnvelopeEncryptorMultiAlgorithm(keksByAlg map[Algorithm]map[int][]byte, current int, currentAlg Algorithm) (*EnvelopeEncryptor, error) {
	if current < 0 {
		return nil, apperrors.NewInvalidArgument("current KEK version must be >= 0")
	}
	currentKeks, ok := keksByAlg[currentAlg]
	if !ok {
		return nil, apperrors.NewInvalidArgumentf("current algorithm %s has no KEK set", algName(currentAlg))
	}
	if _, ok := currentKeks[current]; !ok {
		return nil, apperrors.NewInvalidArgumentf("current KEK version %d not provided", current)
	}

	keySize, err := algorithmKeySize(currentAlg)
	if err != nil {
		return nil, err
	}
	e := &EnvelopeEncryptor{
		keksByAlg: make(map[Algorithm]map[int]cipher.AEAD, len(keksByAlg)),
		current:   current,
		dekSize:   keySize,
		algorithm: currentAlg,
	}

	for alg, keks := range keksByAlg {
		algKeySize, err := algorithmKeySize(alg)
		if err != nil {
			return nil, err
		}
		e.keksByAlg[alg] = make(map[int]cipher.AEAD, len(keks))
		for v, key := range keks {
			if len(key) != algKeySize {
				return nil, apperrors.NewInvalidArgumentf(
					"KEK version %d must be exactly %d bytes for %s, got %d",
					v, algKeySize, algName(alg), len(key))
			}
			block, err := newCipher(alg, key)
			if err != nil {
				return nil, apperrors.WrapInternalf(err, "create %s cipher for KEK version %d", algName(alg), v)
			}
			gcm, err := cipher.NewGCM(block)
			if err != nil {
				return nil, apperrors.WrapInternalf(err, "create GCM for KEK version %d", v)
			}
			e.keksByAlg[alg][v] = gcm
		}
	}

	return e, nil
}

// Algorithm returns the symmetric cipher this encryptor is configured to use.
// Exposed for tests and field_encryption observability.
func (e *EnvelopeEncryptor) Algorithm() Algorithm {
	return e.algorithm
}

// String returns the canonical algorithm name for logs and diagnostics.
func (a Algorithm) String() string {
	return algName(a)
}

// CurrentVersion returns the active KEK version.
func (e *EnvelopeEncryptor) CurrentVersion() int {
	return e.current
}

// HasVersion reports whether the given KEK version is available for decryption
// under the write algorithm.
func (e *EnvelopeEncryptor) HasVersion(v int) bool {
	_, ok := e.keksByAlg[e.algorithm][v]
	return ok
}

// Encrypt wraps plaintext using the v1 envelope scheme (no AAD) with the
// current KEK version. Non-column callers (JSON/config sealing, logout-token
// caches) keep this API; encrypted columns must use EncryptWithAAD so the
// ciphertext is bound to its row.
func (e *EnvelopeEncryptor) Encrypt(plaintext string) (string, error) {
	return e.encrypt(plaintext, nil)
}

// EncryptWithAAD wraps plaintext into a v2 envelope. The same AAD is passed to
// both GCM layers (KEK-wraps-DEK and DEK-encrypts-plaintext); an empty AAD is
// refused instead of silently degrading to v1.
func (e *EnvelopeEncryptor) EncryptWithAAD(plaintext string, aad []byte) (string, error) {
	if len(aad) == 0 {
		return "", apperrors.WrapInternal(ErrAADContext, "v2 envelope requires an AAD context")
	}
	return e.encrypt(plaintext, aad)
}

func (e *EnvelopeEncryptor) encrypt(plaintext string, aad []byte) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	// Generate a fresh DEK
	dek := make([]byte, e.dekSize)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return "", apperrors.WrapInternalf(err, "generate DEK")
	}

	// Encrypt plaintext with DEK (使用 configured algorithm)
	dekBlock, err := newCipher(e.algorithm, dek)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "create DEK cipher")
	}
	dekGCM, err := cipher.NewGCM(dekBlock)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "create DEK GCM")
	}
	nonce := make([]byte, dekGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", apperrors.WrapInternalf(err, "generate DEK nonce")
	}
	ciphertext := dekGCM.Seal(nil, nonce, []byte(plaintext), aad)

	// Wrap DEK with current KEK. The v2 AAD covers both layers; v1 passes nil.
	kekGCM := e.keksByAlg[e.algorithm][e.current]
	kekNonce := make([]byte, kekGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, kekNonce); err != nil {
		return "", apperrors.WrapInternalf(err, "generate KEK nonce")
	}
	encryptedDEK := kekGCM.Seal(kekNonce, kekNonce, dek, aad)

	tag, _ := algorithmTag(e.algorithm)
	aadSuffix := ""
	if aad != nil {
		aadSuffix = "-aad"
	}
	token := fmt.Sprintf("v%d-%s%s:%s:%s:%s",
		e.current,
		tag,
		aadSuffix,
		base64.StdEncoding.EncodeToString(encryptedDEK),
		base64.StdEncoding.EncodeToString(nonce),
		base64.StdEncoding.EncodeToString(ciphertext),
	)
	return token, nil
}

// Decrypt unwraps a versioned envelope token and returns the plaintext.
//
// Discrimination follows ADR-015: a token matching the v2 shape is only
// decrypted as v2 (and fails with ErrAADContext without a row binding), a
// token matching the v1 shape is decrypted as v1, and only a value that
// matches neither shape may use the v0 legacy branch. An authentication
// failure never falls through to a lower generation.
func (e *EnvelopeEncryptor) Decrypt(token string) (string, error) {
	return e.DecryptWithAAD(token, nil)
}

// DecryptWithAAD decrypts v2 tokens with the row binding, v1 tokens without
// it, and v0 legacy values. v2 authentication failures are returned as
// ErrAADMismatch and are never retried as v1 or v0.
func (e *EnvelopeEncryptor) DecryptWithAAD(token string, aad []byte) (string, error) {
	if token == "" {
		return "", nil
	}
	if IsEnvelopeToken(token) {
		return e.decryptEnvelope(token, aad)
	}
	// Legacy v0 format: base64(nonce+ciphertext) encrypted directly with KEK.
	return e.decryptLegacy(token)
}

// decryptEnvelope parses and decrypts a versioned envelope token.
func (e *EnvelopeEncryptor) decryptEnvelope(token string, aad []byte) (string, error) {
	parts := strings.SplitN(token, ":", 4)
	if len(parts) != 4 {
		return "", apperrors.NewInvalidArgument("envelope token must have 4 parts")
	}

	header, err := parseEnvelopeHeader(parts[0])
	if err != nil {
		return "", err
	}

	if header.aad {
		if len(aad) == 0 {
			return "", apperrors.WrapInternal(ErrAADContext, "v2 envelope requires a table/column/primary key binding")
		}
	} else {
		// v1 has no AAD: ignore a caller-supplied binding instead of feeding it
		// to GCM, which would make legacy tokens unreadable.
		aad = nil
	}

	// 新 token 自带算法标识；未标记的历史 token 沿用当前配置算法。
	algorithm := e.algorithm
	if header.tagged {
		algorithm = header.algorithm
	}
	kekSet, ok := e.keksByAlg[algorithm]
	if !ok {
		return "", apperrors.NewInvalidArgumentf(
			"envelope algorithm %s is not configured for decryption (set the compatibility key or re-encrypt before switching)",
			algName(algorithm))
	}
	kekGCM, ok := kekSet[header.version]
	if !ok {
		return "", apperrors.NewInvalidArgumentf("KEK version %d not available for decryption", header.version)
	}

	encryptedDEK, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", apperrors.WrapInternalf(err, "decode encrypted DEK")
	}
	nonce, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", apperrors.WrapInternalf(err, "decode nonce")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return "", apperrors.WrapInternalf(err, "decode ciphertext")
	}

	// Unwrap DEK
	kekNonceSize := kekGCM.NonceSize()
	if len(encryptedDEK) < kekNonceSize {
		return "", apperrors.NewInvalidArgument("encrypted DEK too short")
	}
	kekNonce, wrappedDEK := encryptedDEK[:kekNonceSize], encryptedDEK[kekNonceSize:]
	dek, err := kekGCM.Open(nil, kekNonce, wrappedDEK, aad)
	if err != nil {
		if header.aad {
			return "", apperrors.WrapInternal(ErrAADMismatch, "v2 wrapped DEK authentication failed for the supplied row binding")
		}
		return "", apperrors.WrapInternalf(err, "unwrap DEK with KEK version %d", header.version)
	}

	// Decrypt plaintext with the algorithm selected by the token header.
	dekBlock, err := newCipher(algorithm, dek)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "create DEK cipher")
	}
	dekGCM, err := cipher.NewGCM(dekBlock)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "create DEK GCM")
	}
	plaintext, err := dekGCM.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		if header.aad {
			return "", apperrors.WrapInternal(ErrAADMismatch, "v2 ciphertext authentication failed for the supplied row binding")
		}
		return "", apperrors.WrapInternalf(err, "decrypt with DEK")
	}

	return string(plaintext), nil
}

// decryptLegacy decrypts the pre-envelope format (nonce+ciphertext base64)
// trying each available KEK until one succeeds.
func (e *EnvelopeEncryptor) decryptLegacy(token string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "decode legacy base64")
	}

	// Try current version first, then others. Legacy direct-KEK tokens predate
	// the algorithm tag, so they are bound to the configured write algorithm.
	versions := []int{e.current}
	for v := range e.keksByAlg[e.algorithm] {
		if v != e.current {
			versions = append(versions, v)
		}
	}

	var lastErr error
	for _, v := range versions {
		gcm := e.keksByAlg[e.algorithm][v]
		nonceSize := gcm.NonceSize()
		if len(data) < nonceSize {
			continue
		}
		nonce, ct := data[:nonceSize], data[nonceSize:]
		plaintext, err := gcm.Open(nil, nonce, ct, nil)
		if err == nil {
			return string(plaintext), nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return "", apperrors.WrapInternalf(lastErr, "decrypt legacy token (no KEK version succeeded)")
	}
	return "", apperrors.NewInvalidArgument("ciphertext too short")
}

// Reencrypt keeps the historical no-AAD rotation contract for non-column
// tokens: it migrates KEK versions but never changes the envelope generation.
// Column rotation must use ReencryptWithAAD, which upgrades v1 to v2.
func (e *EnvelopeEncryptor) Reencrypt(token string) (rotated string, changed bool, err error) {
	if token == "" {
		return "", false, nil
	}

	// v1 at the current KEK/algorithm is already as current as the no-AAD API
	// can express; skip it instead of rewriting the token on every pass.
	if e.isCurrentKeyEnvelope(token) {
		return token, false, nil
	}

	plaintext, err := e.Decrypt(token)
	if err != nil {
		return "", false, err
	}

	newToken, err := e.Encrypt(plaintext)
	if err != nil {
		return "", false, err
	}

	return newToken, true, nil
}

// ReencryptWithAAD rewrites a column value into a v2 envelope bound to the
// given AAD. A v2 token already at the current KEK/algorithm is authenticated
// against the AAD and returned unchanged; v1 and v0 values are upgraded to v2.
// This is the rotation primitive ReencryptJob uses so v1 is never treated as
// current.
func (e *EnvelopeEncryptor) ReencryptWithAAD(token string, aad []byte) (rotated string, changed bool, err error) {
	if token == "" {
		return "", false, nil
	}
	if len(aad) == 0 {
		return "", false, apperrors.WrapInternal(ErrAADContext, "v2 envelope requires an AAD context")
	}

	if e.isCurrentEnvelope(token, aad) {
		// Authenticate even when skipping: a ciphertext moved to another row
		// must surface as ErrAADMismatch instead of staying "current".
		if _, err := e.decryptEnvelope(token, aad); err != nil {
			return "", false, err
		}
		return token, false, nil
	}

	plaintext, err := e.DecryptWithAAD(token, aad)
	if err != nil {
		return "", false, err
	}

	newToken, err := e.EncryptWithAAD(plaintext, aad)
	if err != nil {
		return "", false, err
	}

	return newToken, true, nil
}

// isCurrentEnvelope reports whether the token is a v2 envelope at the current
// KEK version and write algorithm. v1 is deliberately never current: RunFor
// must rewrite every v1 token into the AAD-bound generation.
func (e *EnvelopeEncryptor) isCurrentEnvelope(token string, aad []byte) bool {
	if len(aad) == 0 || !isV2EnvelopeToken(token) {
		return false
	}
	headerPart, _, found := strings.Cut(token, ":")
	if !found {
		return false
	}
	header, err := parseEnvelopeHeader(headerPart)
	if err != nil || !header.aad || header.version != e.current {
		return false
	}
	return header.tagged && header.algorithm == e.algorithm
}

// isCurrentKeyEnvelope is the pre-G7 check used only by the no-AAD Reencrypt
// API: it treats untagged v1 tokens at the current version as current so the
// legacy API keeps its idempotent behavior.
func (e *EnvelopeEncryptor) isCurrentKeyEnvelope(token string) bool {
	headerPart, _, found := strings.Cut(token, ":")
	if !found {
		return false
	}
	header, err := parseEnvelopeHeader(headerPart)
	if err != nil || header.version != e.current || header.aad {
		return false
	}
	if !header.tagged {
		return true
	}
	return header.algorithm == e.algorithm
}
