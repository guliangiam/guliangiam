package crypto

import (
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"io"
	"math/big"
	"strings"
	"sync/atomic"

	"github.com/emmansun/gmsm/sm2"
	"github.com/emmansun/gmsm/sm3"
	"github.com/emmansun/gmsm/sm4"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 能力位 ID 常量、声明顺序与 failure_mode 派生清单由
// scripts/gen_crypto_capabilities.py 从 contracts/registries/crypto.yaml 生成，
// 落点是 capabilities_gen.go；本文件只保留手写探测实现与分派表。
// 只有本包（suite.go / gm_tls.go / envelope.go）允许 import
// github.com/emmansun/gmsm；业务流程一律通过 CryptoSuite 使用国密算法。

// Profile 是 CRYPTO_PROFILE 的取值。
type Profile string

const (
	ProfileDefault Profile = "default"
	ProfileGM      Profile = "gm"
)

// HealthStatus 是 /healthz 对外披露的加密能力状态。
// DegradedReasons 携带降级能力位的具体原因（已剔除文件路径），让运维不必
// 翻启动日志才能知道 gm_tls 为什么不可用。
type HealthStatus struct {
	Status          string            `json:"status"`
	Profile         string            `json:"profile"`
	Missing         []string          `json:"missing,omitempty"`
	Degraded        []string          `json:"degraded,omitempty"`
	DegradedReasons map[string]string `json:"degraded_reasons,omitempty"`
}

const (
	HealthStatusOK          = "ok"
	HealthStatusDegraded    = "degraded"
	HealthStatusUnavailable = "unavailable"
)

// CapabilityStatus 是单个能力位的探测结果。
type CapabilityStatus struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
}

// ProfileReport 是启动时能力探测与档位裁决的结果。
type ProfileReport struct {
	Profile      string             `json:"profile"`
	Available    bool               `json:"available"`
	Capabilities []CapabilityStatus `json:"capabilities"`
	Missing      []string           `json:"missing,omitempty"`
	Degraded     []string           `json:"degraded,omitempty"`
}

var runtimeHealth atomic.Value // HealthStatus

func init() {
	runtimeHealth.Store(HealthStatus{Status: HealthStatusOK, Profile: string(ProfileDefault)})
}

// detectCapabilities 是启动探测的可替换入口：生产恒为 DetectCapabilities，
// 测试替换它以证明 gm 档缺能力时 ValidateRuntimeProfile 拒绝启动。
var detectCapabilities = DetectCapabilities

// ValidateRuntimeProfile 读取 CRYPTO_PROFILE 并执行启动探测。
// gm 档缺少 sm2_sign / sm2_encrypt / sm3 / sm4 时返回错误（拒绝启动，禁止降级）；
// gm_tls 缺失只进入 Degraded，不阻断启动。
func ValidateRuntimeProfile() (ProfileReport, error) {
	return AssessProfile(config.CryptoProfile(), detectCapabilities())
}

// AssessProfile 是能力裁决入口：调用方注入探测结果，便于测试缺失路径。
// 除能力位外，它还校验 gm 档与 ENCRYPTION_ALGORITHM 的一致性：gm 档禁止显式
// AES（必须省略以默认 sm4-gcm，或显式 sm4-gcm），否则返回结构化错误让
// bootstrap.applyCryptoProfile 在创建监听器前拒绝启动。
func AssessProfile(profile string, capabilities []CapabilityStatus) (ProfileReport, error) {
	normalized := strings.ToLower(strings.TrimSpace(profile))
	if normalized == "" {
		normalized = string(ProfileDefault)
	}
	switch Profile(normalized) {
	case ProfileDefault, ProfileGM:
	default:
		return ProfileReport{}, apperrors.NewInvalidArgumentf(
			"unsupported CRYPTO_PROFILE %q; supported values: default, gm", profile)
	}

	byID := make(map[string]CapabilityStatus, len(capabilities))
	for _, capability := range capabilities {
		byID[capability.ID] = capability
	}

	report := ProfileReport{
		Profile:      normalized,
		Available:    true,
		Capabilities: orderedCapabilities(byID),
	}
	if Profile(normalized) == ProfileGM {
		if configured := config.EncryptionAlgorithm(); isAESAlgorithm(configured) {
			report.Available = false
			return report, apperrors.NewInvalidArgumentf(
				"CRYPTO_PROFILE=gm forbids ENCRYPTION_ALGORITHM=%q: unset it to default to sm4-gcm or set sm4-gcm explicitly; refusing to start (no silent downgrade to AES)",
				configured)
		}
		for _, id := range gmRequiredCapabilities {
			if !capabilityAvailable(byID, id) {
				report.Missing = append(report.Missing, id)
			}
		}
		for _, id := range gmDegradableCapabilities {
			if !capabilityAvailable(byID, id) {
				report.Degraded = append(report.Degraded, id)
			}
		}
	}
	if len(report.Missing) > 0 {
		report.Available = false
		return report, apperrors.NewUnavailablef(
			"CRYPTO_PROFILE=gm requires capabilities that are unavailable: %s; refusing to start (no silent downgrade to international algorithms)",
			strings.Join(report.Missing, ", "))
	}
	return report, nil
}

func capabilityAvailable(byID map[string]CapabilityStatus, id string) bool {
	status, ok := byID[id]
	return ok && status.Available
}

func orderedCapabilities(byID map[string]CapabilityStatus) []CapabilityStatus {
	out := make([]CapabilityStatus, 0, len(registryCapabilityOrder))
	for _, id := range registryCapabilityOrder {
		if status, ok := byID[id]; ok {
			out = append(out, status)
			continue
		}
		out = append(out, CapabilityStatus{ID: id, Available: false, Detail: "probe did not report this capability"})
	}
	return out
}

// SetRuntimeReport 把启动裁决结果发布给 /healthz。启动失败时进程不会提供服务，
// 因此这里只处理“可用”与“降级”两种运行态。
func SetRuntimeReport(report ProfileReport) {
	status := HealthStatusOK
	if !report.Available {
		status = HealthStatusUnavailable
	} else if len(report.Degraded) > 0 {
		status = HealthStatusDegraded
	}
	reasons := make(map[string]string, len(report.Degraded))
	for _, id := range report.Degraded {
		for _, capability := range report.Capabilities {
			if capability.ID == id && capability.Detail != "" {
				reasons[id] = capability.Detail
				break
			}
		}
	}
	runtimeHealth.Store(HealthStatus{
		Status:          status,
		Profile:         report.Profile,
		Missing:         append([]string(nil), report.Missing...),
		Degraded:        append([]string(nil), report.Degraded...),
		DegradedReasons: reasons,
	})
}

// HealthReport 返回进程级加密能力状态，供 /healthz 序列化。
func HealthReport() HealthStatus {
	status, ok := runtimeHealth.Load().(HealthStatus)
	if !ok {
		return HealthStatus{Status: HealthStatusOK, Profile: string(ProfileDefault)}
	}
	if len(status.DegradedReasons) > 0 {
		reasons := make(map[string]string, len(status.DegradedReasons))
		for id, reason := range status.DegradedReasons {
			reasons[id] = reason
		}
		status.DegradedReasons = reasons
	}
	return status
}

// capabilityDetectors 是能力位 → 探测实现的唯一分派表。键必须与生成物
// registryCapabilityOrder 一一对应：capabilities_test.go 双向断言，防止
// 「注册表加了能力位但忘记写探测」或「探测表残留已删除的能力位」。
var capabilityDetectors = map[string]func() CapabilityStatus{
	CapabilitySM2Sign: func() CapabilityStatus {
		return probe(CapabilitySM2Sign, "SM2 sign/verify self-test", probeSM2Sign)
	},
	CapabilitySM2Encrypt: func() CapabilityStatus {
		return probe(CapabilitySM2Encrypt, "SM2 encrypt/decrypt self-test", probeSM2Encrypt)
	},
	CapabilitySM3: func() CapabilityStatus {
		return probe(CapabilitySM3, "SM3 standard vector", probeSM3)
	},
	CapabilitySM4:   sm4Capability,
	CapabilityGMTLS: gmTLSCapability,
}

// DetectCapabilities 执行一次启动自检。SM2 使用生成密钥 + 签名/验签 +
// 加解密闭环；SM3 使用 GB/T 32905 的 "abc" 标准向量；SM4 使用 GCM 闭环。
// 输出顺序与 /healthz 展示顺序都来自生成物 registryCapabilityOrder。
func DetectCapabilities() []CapabilityStatus {
	out := make([]CapabilityStatus, 0, len(registryCapabilityOrder))
	for _, id := range registryCapabilityOrder {
		detect, ok := capabilityDetectors[id]
		if !ok {
			out = append(out, CapabilityStatus{
				ID:        id,
				Available: false,
				Detail:    "capability detector is not implemented",
			})
			continue
		}
		out = append(out, detect())
	}
	return out
}

func probe(id, detail string, fn func() error) CapabilityStatus {
	status := CapabilityStatus{ID: id, Available: true, Detail: detail}
	if err := fn(); err != nil {
		status.Available = false
		status.Detail = err.Error()
	}
	return status
}

func gmTLSCapability() CapabilityStatus {
	if err := probeGMTLS(); err != nil {
		return CapabilityStatus{ID: CapabilityGMTLS, Available: false, Detail: err.Error()}
	}
	return CapabilityStatus{
		ID:        CapabilityGMTLS,
		Available: true,
		Detail:    "SM2 sign/encrypt certificates, matching private keys and CA chains verified",
	}
}

const (
	sm3ABCVector = "66c7f0f462eeedd9d1f2d46bdc10e4e24167c4875cf2f7a2297da02b8f4ba8e0"
	probeMessage = "guliangiam-gm-capability-selftest"
)

func probeSM3() error {
	got := hex.EncodeToString(SM3Sum([]byte("abc")))
	if got != sm3ABCVector {
		return apperrors.NewInternal("SM3 self-test vector mismatch")
	}
	return nil
}

func probeSM4() error {
	key := make([]byte, 16)
	block, err := sm4.NewCipher(key)
	if err != nil {
		return apperrors.WrapInternal(err, "SM4 self-test cipher")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return apperrors.WrapInternal(err, "SM4 self-test GCM")
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return apperrors.WrapInternal(err, "SM4 self-test nonce")
	}
	sealed := gcm.Seal(nil, nonce, []byte(probeMessage), nil)
	opened, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil || string(opened) != probeMessage {
		return apperrors.NewInternal("SM4-GCM self-test roundtrip failed")
	}
	return nil
}

// sm4CapabilityProbe 是可注入的 SM4 能力探测 seam：启动能力探测
// （capabilityDetectors[sm4]）与 ENCRYPTION_ALGORITHM=sm4-gcm 的启用判定
// （sm4Available）消费同一次判定，避免同一能力出现两套可用性口径。
// 测试替换它来证明 SM4 不可用时会拒绝启用而不是静默降级。
var sm4CapabilityProbe = probeSM4

func sm4Capability() CapabilityStatus {
	return probe(CapabilitySM4, "SM4-GCM self-test", sm4CapabilityProbe)
}

// sm4Available 是字段加密算法选择对 SM4 能力的唯一判定入口（ADR-031
// Decision 4：启动探测与披露）。判定实现就是 probeSM4 的 GCM 闭环自检。
func sm4Available() bool {
	return sm4CapabilityProbe() == nil
}

func probeSM2Sign() error {
	priv, err := GenerateSM2Key()
	if err != nil {
		return err
	}
	digest := SM3Sum([]byte(probeMessage))
	signature, err := NewCryptoSuite().SM2Sign(priv, digest)
	if err != nil {
		return err
	}
	if !NewCryptoSuite().SM2Verify(priv.Public(), digest, signature) {
		return apperrors.NewInternal("SM2 sign self-test verification failed")
	}
	return nil
}

func probeSM2Encrypt() error {
	priv, err := GenerateSM2Key()
	if err != nil {
		return err
	}
	ciphertext, err := NewCryptoSuite().SM2Encrypt(priv.Public(), []byte(probeMessage))
	if err != nil {
		return err
	}
	plaintext, err := NewCryptoSuite().SM2Decrypt(priv, ciphertext)
	if err != nil || string(plaintext) != probeMessage {
		return apperrors.NewInternal("SM2 encrypt self-test roundtrip failed")
	}
	return nil
}

// SM2PrivateKey 是 CryptoSuite 的不透明私钥句柄，业务流程无需 import gmsm。
type SM2PrivateKey struct {
	key *sm2.PrivateKey
}

// SM2PublicKey 是 CryptoSuite 的不透明公钥句柄。
type SM2PublicKey struct {
	key *ecdsa.PublicKey
}

// Public 返回对应的公钥句柄。
func (k *SM2PrivateKey) Public() *SM2PublicKey {
	if k == nil || k.key == nil {
		return nil
	}
	return &SM2PublicKey{key: &k.key.PublicKey}
}

// Coordinates returns the SM2 public point (x, y) for a JWK EC representation.
// It returns false for a nil key so callers fail closed instead of panicking.
func (k *SM2PublicKey) Coordinates() (x, y *big.Int, ok bool) {
	if k == nil || k.key == nil {
		return nil, nil, false
	}
	return k.key.X, k.key.Y, true
}

// CryptoSuite 把 SM2/SM3/SM4 收口到平台层；业务代码只依赖本类型。
//
//nolint:revive // CryptoSuite 是 ADR-031/E0a 约定的公共契约名，与注册表和文档保持一致。
type CryptoSuite struct{}

func NewCryptoSuite() CryptoSuite {
	return CryptoSuite{}
}

func GenerateSM2Key() (*SM2PrivateKey, error) {
	key, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate SM2 key")
	}
	return &SM2PrivateKey{key: key}, nil
}

func (CryptoSuite) SM2Sign(privateKey *SM2PrivateKey, digest []byte) ([]byte, error) {
	if privateKey == nil || privateKey.key == nil {
		return nil, apperrors.NewInvalidArgument("SM2 private key is required")
	}
	if len(digest) == 0 {
		return nil, apperrors.NewInvalidArgument("SM2 sign digest is required")
	}
	// SM2 签名要求调用方传入摘要（通常是 SM3 输出）；这里不做隐式哈希，避免
	// 出现“传原文却被当摘要”的静默错用。
	signature, err := sm2.SignASN1(rand.Reader, privateKey.key, digest, nil)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "SM2 sign")
	}
	return signature, nil
}

func (CryptoSuite) SM2Verify(publicKey *SM2PublicKey, digest, signature []byte) bool {
	if publicKey == nil || publicKey.key == nil || len(digest) == 0 || len(signature) == 0 {
		return false
	}
	return sm2.VerifyASN1(publicKey.key, digest, signature)
}

func (CryptoSuite) SM2Encrypt(publicKey *SM2PublicKey, plaintext []byte) ([]byte, error) {
	if publicKey == nil || publicKey.key == nil {
		return nil, apperrors.NewInvalidArgument("SM2 public key is required")
	}
	ciphertext, err := sm2.Encrypt(rand.Reader, publicKey.key, plaintext, nil)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "SM2 encrypt")
	}
	return ciphertext, nil
}

func (CryptoSuite) SM2Decrypt(privateKey *SM2PrivateKey, ciphertext []byte) ([]byte, error) {
	if privateKey == nil || privateKey.key == nil {
		return nil, apperrors.NewInvalidArgument("SM2 private key is required")
	}
	plaintext, err := sm2.Decrypt(privateKey.key, ciphertext)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "SM2 decrypt")
	}
	return plaintext, nil
}

func SM3Sum(data []byte) []byte {
	hash := sm3.New()
	_, _ = hash.Write(data)
	return hash.Sum(nil)
}

func (CryptoSuite) SM3Sum(data []byte) []byte {
	return SM3Sum(data)
}

func (CryptoSuite) SM4GCMSeal(key, nonce, plaintext, additionalData []byte) ([]byte, error) {
	gcm, err := sm4GCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, apperrors.NewInvalidArgumentf("SM4-GCM nonce must be %d bytes", gcm.NonceSize())
	}
	return gcm.Seal(nil, nonce, plaintext, additionalData), nil
}

func (CryptoSuite) SM4GCMOpen(key, nonce, ciphertext, additionalData []byte) ([]byte, error) {
	gcm, err := sm4GCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, apperrors.NewInvalidArgumentf("SM4-GCM nonce must be %d bytes", gcm.NonceSize())
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, additionalData)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "SM4-GCM open")
	}
	return plaintext, nil
}

func sm4GCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 16 {
		return nil, apperrors.NewInvalidArgumentf("SM4 key must be 16 bytes, got %d", len(key))
	}
	block, err := sm4.NewCipher(key)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "create SM4 cipher")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "create SM4-GCM")
	}
	return gcm, nil
}
