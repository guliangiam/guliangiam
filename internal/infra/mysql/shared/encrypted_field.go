package shared

import (
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

// EncryptedFieldMapper 代理 process-wide field encryptor，给持久化敏感列的 repo 使用。
// 懒加载：首次调用才解析 encryptor，避免 package init 因缺少 ENCRYPTION_KEY_CURRENT panic。
//
// 所有方法都要求 crypto.AADContext：写路径一律 v2（table/column/主键绑定），
// 读路径用同一 AAD 认证。这里不提供无 AAD 的 Encrypt/Decrypt，避免新 mapper
// 静默退回 v1（ADR-015 GM-G7）。
type EncryptedFieldMapper struct{}

func NewEncryptedFieldMapper() *EncryptedFieldMapper {
	return &EncryptedFieldMapper{}
}

// EncryptColumn 把新值封成 v2 信封；缺少主键上下文直接报错，不退化为 v1。
func (m *EncryptedFieldMapper) EncryptColumn(aad crypto.AADContext, value string) (string, error) {
	return crypto.GlobalFieldEncryptor().EncryptColumn(aad, value)
}

// DecryptColumn 按 v2（行绑定认证）→v1→v0 解密。
func (m *EncryptedFieldMapper) DecryptColumn(aad crypto.AADContext, value string) (string, error) {
	return crypto.GlobalFieldEncryptor().DecryptColumn(aad, value)
}

// DecryptEnvelopeColumn 只接受 envelope 令牌（v2/v1）；明文与 legacy v0 返回
// crypto.ErrNotEnvelope，v2 认证失败不回退。
func (m *EncryptedFieldMapper) DecryptEnvelopeColumn(aad crypto.AADContext, value string) (string, error) {
	return crypto.GlobalFieldEncryptor().DecryptEnvelopeFieldWithAAD(aad, value)
}

func (m *EncryptedFieldMapper) IsEnabled() bool {
	return crypto.GlobalFieldEncryptor().IsEnabled()
}

// RotateColumn 把值重封为当前 KEK 的 v2 代次；重复执行幂等。
func (m *EncryptedFieldMapper) RotateColumn(aad crypto.AADContext, value string) (rotated string, changed bool, err error) {
	return crypto.GlobalFieldEncryptor().RotateColumn(aad, value)
}

func (m *EncryptedFieldMapper) HasPreviousKey() bool {
	return crypto.GlobalFieldEncryptor().HasPreviousKey()
}
