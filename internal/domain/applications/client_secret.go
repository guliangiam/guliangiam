package applications

import (
	"strings"
	"time"
)

// RFC 7591 §2 的 token_endpoint_auth_method 取值。none / basic / post 的常量在
// application.go 中保留历史定义，这里补齐其余三种（RFC 7523 / RFC 8705）。
//
// client_secret_jwt 不在此闭集：本仓库没有可用的数据库托管密钥服务，HS256
// 验签所需原始 secret 无处安全存放（OPEN-API-60）。该常量仅用于显式拒绝。
const (
	TokenEndpointAuthMethodClientSecretJWT         = "client_secret_jwt"
	TokenEndpointAuthMethodPrivateKeyJWT           = "private_key_jwt"
	TokenEndpointAuthMethodTLSClientAuth           = "tls_client_auth"
	TokenEndpointAuthMethodSelfSignedTLSClientAuth = "self_signed_tls_client_auth"
)

// TokenEndpointAuthMethods 是注册接口接受的全部取值（顺序即前端展示顺序）。
// 发现文档的 token_endpoint_auth_methods_supported 必须与真实验证器一致，
// 由实现显式声明，不能直接复用本切片。
var TokenEndpointAuthMethods = []string{
	TokenEndpointAuthMethodNone,
	TokenEndpointAuthMethodClientSecretBasic,
	TokenEndpointAuthMethodClientSecretPost,
	TokenEndpointAuthMethodPrivateKeyJWT,
	TokenEndpointAuthMethodTLSClientAuth,
	TokenEndpointAuthMethodSelfSignedTLSClientAuth,
}

// IsRegisteredTokenEndpointAuthMethod 判断取值是否属于注册闭集。
func IsRegisteredTokenEndpointAuthMethod(method string) bool {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case TokenEndpointAuthMethodNone,
		TokenEndpointAuthMethodClientSecretBasic,
		TokenEndpointAuthMethodClientSecretPost,
		TokenEndpointAuthMethodPrivateKeyJWT,
		TokenEndpointAuthMethodTLSClientAuth,
		TokenEndpointAuthMethodSelfSignedTLSClientAuth:
		return true
	default:
		return false
	}
}

// ClientSecretStatus 是 app_client_secrets.status 的域枚举。
//
// active：唯一可用于签发的当前 secret；
// grace：刚被轮换替换，仍在 grace_until 窗口内可验签；
// retired：窗口结束或被显式停用，永久不可用（行保留以便审计回滚）。
type ClientSecretStatus string

const (
	ClientSecretStatusActive  ClientSecretStatus = "active"
	ClientSecretStatusGrace   ClientSecretStatus = "grace"
	ClientSecretStatusRetired ClientSecretStatus = "retired"
)

// ClientSecret 是 app_client_secrets 的一行。
//
// SecretHash 是 argon2id 哈希，供 client_secret_basic / client_secret_post 验签；
// 表中不存任何可直接还原明文 secret 的材料（client_secret_jwt 未实现）。
type ClientSecret struct {
	ID             string
	ApplicationID  string
	OrganizationID string
	SecretPrefix   string
	SecretHash     string
	Status         ClientSecretStatus
	RotatedAt      *time.Time
	GraceUntil     *time.Time
	RetiredAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// UsableAt 报告该 secret 在 now 时刻是否可用于验签。
//
// 轮换判定发生在验签时（无定时任务）：grace 行只有在 grace_until 之后才失效。
func (s ClientSecret) UsableAt(now time.Time) bool {
	switch s.Status {
	case ClientSecretStatusActive:
		return true
	case ClientSecretStatusGrace:
		return s.GraceUntil != nil && s.GraceUntil.After(now)
	default:
		return false
	}
}

// Masked 返回可安全展示的前缀。secret_prefix 已由生成器截取为
// "gl_cs_" + 前 6 个随机字符；为空时回退到固定的占位串，绝不回显哈希。
func (s ClientSecret) Masked() string {
	if strings.TrimSpace(s.SecretPrefix) == "" {
		return ""
	}
	return strings.TrimSpace(s.SecretPrefix) + "…"
}
