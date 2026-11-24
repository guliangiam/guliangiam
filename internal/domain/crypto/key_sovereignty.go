// Package crypto 定义 ADR-031 Decision 3 密钥主权的域模型。
//
// 设计原则（Experience 100007470 安全校验分层）：
//   - KeyRevokedError: 加密层拒绝（新加密不能用已吊销的 KEK）
//   - EnvelopeDecryptError: 解密失败（结构层）
//   - 两者绝不混用：加密时只关心"能否加密"，解密时只关心"能否解开"
//   - 吊销后数据不可读语义 = 加密拒绝（不产生新密文）+ 保留旧密文解密能力
package crypto

import (
	"context"
	"errors"
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// KeySovereigntyMode 是 ADR-031 Decision 3 的密钥主权模式。
type KeySovereigntyMode string

const (
	// SovereigntyPlatformDefault: 平台持有 KEK（默认，向后兼容）
	SovereigntyPlatformDefault KeySovereigntyMode = "platform-default"

	// SovereigntyBYOK: Bring Your Own Key
	// 租户自己生成 KEK，提供给平台；平台 KMS 信封托管。
	// key_material_ref = KMS key ARN；平台持有 DEK（信封结构不变）。
	SovereigntyBYOK KeySovereigntyMode = "byok"

	// SovereigntyHYOK: Hold Your Own Key
	// KEK 永远不离开租户侧 KMS/HSM；平台只持引用（URI），
	// 所有 KEK 操作（wrapping/unwrapping）必须回租户侧做。
	SovereigntyHYOK KeySovereigntyMode = "hyok"
)

// String 返回可读名。
func (m KeySovereigntyMode) String() string { return string(m) }

// sovereigntyModes / keyScopes are the closed sets the checks and the 422
// `allowed` metadata share.
var sovereigntyModes = []string{
	string(SovereigntyPlatformDefault), string(SovereigntyBYOK), string(SovereigntyHYOK),
}

var keyScopes = []string{
	string(ScopeGlobal), string(ScopeRegion), string(ScopeOrganization),
}

// Validate 校验主权模式合法。
func (m KeySovereigntyMode) Validate() error {
	switch m {
	case SovereigntyPlatformDefault, SovereigntyBYOK, SovereigntyHYOK:
		return nil
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("unknown sovereignty mode: %s", m),
			map[string]any{"field": "sovereigntyMode", "value": string(m), "allowed": append([]string{}, sovereigntyModes...)},
		)
	}
}

// KeyScope 是 encryption_keys.tenant_scope 的域枚举。
type KeyScope string

const (
	// ScopeGlobal: 平台级 KEK — 所有 org 默认用
	ScopeGlobal KeyScope = "global"

	// ScopeRegion: 区域级 KEK — 只有该 region_code 的 org 用
	ScopeRegion KeyScope = "region"

	// ScopeOrganization: org 级 KEK — 只有该 org 自己用（BYOK dedicated org）
	ScopeOrganization KeyScope = "organization"
)

// Validate 校验作用域合法。
func (s KeyScope) Validate() error {
	switch s {
	case ScopeGlobal, ScopeRegion, ScopeOrganization:
		return nil
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("unknown key scope: %s", s),
			map[string]any{"field": "scope", "value": string(s), "allowed": append([]string{}, keyScopes...)},
		)
	}
}

// EncryptionKey 是 ADR-031 Decision 3 的加密密钥域模型。
// 对应 DB 表 encryption_keys（V5 migration 扩展后）。
//
// 重要：KeyMaterial **从不** 写入 DB — 只存 KeyMaterialRef（引用）和 KeyFingerprint（SHA-256 指纹）。
// BYOK:   KeyMaterialRef = KMS key ARN（例如 arn:aws:kms:us-east-1:123456789012:key/...）
// HYOK:   KeyMaterialRef = 租户侧 HSM URI（例如 hsm://tenant.example.com:29418/key-id=...）
// 默认:   KeyMaterialRef = "env:ENCRYPTION_KEY_CURRENT"
type EncryptionKey struct {
	ID              string
	KeyID           string
	KeyVersion      int
	Algorithm       string
	Purpose         string   // "field-encryption" / "outbox-wrap" / "jwt-sign"
	Status          string   // pending / active / retiring / retired / revoked
	KeyMaterialRef  string   // 密钥引用（永远不是明文！）
	KeyFingerprint  string   // SHA-256 指纹，用于快速检测轮换
	TenantScope     KeyScope // global / region / organization
	ScopeID         string   // region_code 或 org_id（global 时空）
	RegionCode      string   // 冗余列，ScopeRegion 时等于 ScopeID
	SovereigntyMode KeySovereigntyMode
	RevokedAt       *time.Time
	RetiredAt       *time.Time
	ActivatedAt     *time.Time
	RotateAfter     *time.Time
}

// IsUsableForEncrypt 检查 KEK 是否处于可用于**新加密**的状态。
// DR-003 吊销后语义：revoked 状态的 KEK **绝对不能**用于加密；
// 但仍可用于解密旧 token（旧 token 可能被历史 DEK 加密过）。
func (k *EncryptionKey) IsUsableForEncrypt() bool {
	if k == nil {
		return false
	}
	switch k.Status {
	case "active":
		return true
	case "retiring":
		// retiring 期间可能还有新加密（平滑过渡到新版本）；
		// 生产环境严格的话可以改成 false，让调用方只从 active 的列表选。
		return true
	default: // pending / retired / revoked
		return false
	}
}

// IsUsableForDecrypt 检查 KEK 是否处于可用于解密旧 token 的状态。
// 吊销后仍需能解密旧 token — 所以 revoked 也返回 true。
func (k *EncryptionKey) IsUsableForDecrypt() bool {
	if k == nil {
		return false
	}
	switch k.Status {
	case "active", "retiring", "retired", "revoked":
		return true
	default: // pending
		return false
	}
}

// KeyRevokedError 由加密层在 KEK 已吊销时返回。
// Experience 100007470: 这个错误只出现在**加密**入口 —
// 解密入口如果 token 被篡改或者 DEK 用错 KEK 解不开，
// 那是 EnvelopeDecryptError（结构层），绝不混淆。
type KeyRevokedError struct {
	KeyID      string
	KeyVersion int
	RevokedAt  time.Time
}

func (e *KeyRevokedError) Error() string {
	return fmt.Sprintf("encryption key %s v%d has been revoked at %s — cannot use for new encryption",
		e.KeyID, e.KeyVersion, e.RevokedAt.Format(time.RFC3339))
}

// _ 实现 error 接口
var _ error = (*KeyRevokedError)(nil)

// ErrDecryptionKeyNotFound 由解密层在找不到可用 KEK 版本时返回。
// Experience 100007470: 这是"找不到密钥"的语义错误，不是"token 被篡改"的结构错误。
// 调用方决定回滚旧版本还是报给用户。
var ErrDecryptionKeyNotFound = errors.New("decryption key version not available — key may have been permanently deleted")

// EncryptionKeyRepository 是 DB-backed encryption_keys 查询接口。
// infra/mysql + infra/postgres 实现；测试用 StubEncryptionKeyRepo。
type EncryptionKeyRepository interface {
	// ListActiveForOrg 列出 org 可用的所有 active KEK（用于解密 fallback）。
	// 按优先级排序：organization 级 > region 级 > global 级
	ListActiveForOrg(ctx context.Context, orgID string) ([]*EncryptionKey, error)

	// ListForDecrypt 列出可用于解密的 KEK（active/retiring/retired/revoked 都算）。
	ListForDecrypt(ctx context.Context, orgID string) ([]*EncryptionKey, error)

	// GetByVersion 按 purpose + key_version 精确查（token 里带 version 时用）。
	GetByVersion(ctx context.Context, purpose string, keyVersion int) (*EncryptionKey, error)
}
