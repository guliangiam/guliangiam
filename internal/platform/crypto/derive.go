package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"hash"
	"strings"

	"github.com/emmansun/gmsm/sm3"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 派生算法取值与 organization_security_policies.password_derive_algorithm
// 的 CHECK 闭集一致。hmac_sm3 依赖 contracts/registries/crypto.yaml 的
// sm3 能力位（CRYPTO_PROFILE=gm 时缺失会拒绝启动）。
const (
	DeriveAlgorithmHMACSHA256 = "hmac_sha256"
	DeriveAlgorithmHMACSM3    = "hmac_sm3"
)

// DeriveKey 计算键控派生摘要（HMAC 构造），用于一次性口令、短时凭据等
// 需要把明文映射为不可逆标识的场景。它不是密码存储哈希：密码存储固定
// argon2id，SM3 只进入派生口径。
func DeriveKey(algorithm string, key, data []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, apperrors.NewInvalidArgument("derive key must not be empty")
	}

	var mac hash.Hash
	switch strings.TrimSpace(algorithm) {
	case DeriveAlgorithmHMACSHA256:
		mac = hmac.New(sha256.New, key)
	case DeriveAlgorithmHMACSM3:
		mac = hmac.New(sm3.New, key)
	default:
		return nil, apperrors.NewInvalidArgumentf(
			"unsupported derive algorithm %q; supported: %s, %s",
			algorithm, DeriveAlgorithmHMACSHA256, DeriveAlgorithmHMACSM3)
	}

	mac.Write(data) // hash.Hash.Write 不会返回错误。
	return mac.Sum(nil), nil
}
