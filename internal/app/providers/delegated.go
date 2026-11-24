package providers

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 专用子系统（LDAP / 验证码 / Face ID）的托管端点。统一 /providers 对历史行
// 只保留 list/get/disable/delete；create、kind/providerType 变更、activate、
// rotate secret 与 test-connection 一律 422，并在 meta.managedBy 回指这些端点。
const (
	managedByLDAP    = "/api/v1/ldaps"
	managedByCaptcha = "/api/v1/captcha-providers"
	managedByFaceID  = "/api/v1/face-id-providers"

	metaKeyManagedBy = "managedBy"
)

// delegatedIdentityKinds 是「运行期由专用表承载、统一 providers 行只留历史」
// 的 identity kind → 专用管理端点映射。
//
// 这是代码侧唯一声明；contracts/registries/provider-kinds.yaml 用
// managed_via / consumer_table 表达同一事实，scripts/check-provider-kinds.py
// 逐项校验两边（缺项、多项或归属不一致即 FAIL）。不要在别处再手写副本。
//
// schema 闭集不收缩：这些 kind 仍可读，历史行仍可 list/get/disable/delete；
// 只是统一入口不再创建、改类、激活、轮换密钥或做连接测试。专用表与
// LoginWithLDAP / captcha / faceid 登录链路保持原样。
var delegatedIdentityKinds = map[domain.Kind]string{
	"ldap":             managedByLDAP,
	"recaptcha":        managedByCaptcha,
	"recaptcha_v3":     managedByCaptcha,
	"hcaptcha":         managedByCaptcha,
	"turnstile":        managedByCaptcha,
	"geetest":          managedByCaptcha,
	"aliyun_captcha":   managedByCaptcha,
	"tencent_captcha":  managedByCaptcha,
	"faceid":           managedByFaceID,
	"aliyun_face_body": managedByFaceID,
}

func newDelegatedKindError(kind domain.Kind, managedBy string) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"identity provider kind "+string(kind)+" is managed by "+managedBy,
		map[string]any{
			metaKeyField:     metaKeyKind,
			metaKeyValue:     string(kind),
			metaKeyManagedBy: managedBy,
		},
	)
}

// rejectDelegatedKind returns 422 when the kind is owned by a dedicated subsystem.
func rejectDelegatedKind(kind domain.Kind) error {
	if managedBy, ok := delegatedIdentityKinds[kind]; ok {
		return newDelegatedKindError(kind, managedBy)
	}
	return nil
}

// delegatedIdentityOf resolves whether the provider already belongs to a
// dedicated subsystem, through either its registration kind or its
// providerType. A historical row may carry a drifted kind, so both are checked.
func delegatedIdentityOf(provider *Provider) (domain.Kind, string, bool) {
	if provider == nil || provider.Registration.Family != domain.FamilyIdentity {
		return "", "", false
	}
	if managedBy, ok := delegatedIdentityKinds[provider.Registration.Kind]; ok {
		return provider.Registration.Kind, managedBy, true
	}
	if detail, ok := provider.Detail.(*domain.Provider); ok && detail.ProviderType != "" {
		kind := canonicalIdentityKind(string(detail.ProviderType))
		if managedBy, ok := delegatedIdentityKinds[kind]; ok {
			return kind, managedBy, true
		}
	}
	return "", "", false
}

// rejectDelegatedIdentityProvider rejects lifecycle operations on a row whose
// kind or providerType resolves to a dedicated subsystem. Create, activate,
// rotate-secret and test-connection use this; patch adds a change-aware check so
// a non-kind edit of an existing historical row stays possible.
func rejectDelegatedIdentityProvider(provider *Provider) error {
	if kind, managedBy, ok := delegatedIdentityOf(provider); ok {
		return newDelegatedKindError(kind, managedBy)
	}
	return nil
}
