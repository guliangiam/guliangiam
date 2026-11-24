package regions

import (
	"context"

	domaincrypto "github.com/guliangiam/guliangiam/internal/domain/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// RegionKeyResolver 是 ADR-031 Decision 3 的 KEK 区域路由接口。
//
// 给定一个 orgID，返回该 org 所在 region 的**可用 KEK 集合**：
//
//	优先级: org 级 > region 级 > global 级
//
// 实现位置：
//   - infra/mysql/crypto + infra/postgres/crypto: 从 encryption_keys 表 JOIN tenant_instances + deployment_regions
//   - 测试用 StubRegionKeyResolver
type RegionKeyResolver interface {
	// ResolveKEKs 返回 org 的所有可用 KEK（用于 Decrypt fallback）。
	// key: purpose ("field-encryption") → EncryptionKey 列表（按 version ASC）
	ResolveKEKs(ctx context.Context, orgID string, purpose string) (keysOut map[int]*domaincrypto.EncryptionKey, versionOut int, err error)
	//                                                          keks_map, current_version

	// CurrentKEKVersion 返回 org 正在用的 active KEK version。
	CurrentKEKVersion(ctx context.Context, orgID string, purpose string) (int, error)
}

// RegionFieldEncryptor 是 FieldEncryptor 的**按 orgID 动态选 KEK** 版本。
//
// ADR-031 Decision 3 核心：一个 org 的 FieldEncryptor 不再是单一实例（env 读出来的），
// 而是 RegionKeyResolver + KEK 缓存的组合 — 每个 org 根据 region_code 路由到自己的 KEK。
//
// 调用方式：
//
//	fe, err := RegionFieldEncryptorForOrg(ctx, resolver, "org-abc-123", "field-encryption")
//	token, err := fe.EncryptField("credit_card")
//
// DR-003 语义：如果该 org 绑定的 region 没有可用 KEK → 返回 NotFound，
// 不能静默 fallback 到 global（跨 region 数据泄露风险）。
type RegionFieldEncryptor struct {
	resolver  RegionKeyResolver
	orgID     string
	purpose   string
	algorithm string // "AES-256-GCM" 或 "SM4-GCM"
}

// NewRegionFieldEncryptor 构造一个带 region KEK 路由的 FieldEncryptor。
func NewRegionFieldEncryptor(resolver RegionKeyResolver, orgID, purpose, algorithm string) *RegionFieldEncryptor {
	return &RegionFieldEncryptor{
		resolver:  resolver,
		orgID:     orgID,
		purpose:   purpose,
		algorithm: algorithm,
	}
}

// KEKs 返回 org 可用的 KEK version → EncryptionKey 映射。
// 加密入口在拿到 EncryptionKey 后，必须先检查 IsUsableForEncrypt() —
// Experience 100007470 安全分层：加密层检查 KeyRevoked，解密层检查结构合法。
func (rfe *RegionFieldEncryptor) KEKs(ctx context.Context) (keysOut map[int]*domaincrypto.EncryptionKey, versionOut int, err error) {
	keks, currentVersion, err := rfe.resolver.ResolveKEKs(ctx, rfe.orgID, rfe.purpose)
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "resolve KEKs for org %s", rfe.orgID)
	}
	if len(keks) == 0 {
		return nil, 0, apperrors.NewNotFoundf("no available KEKs for org %s purpose %s", rfe.orgID, rfe.purpose)
	}
	return keks, currentVersion, nil
}
