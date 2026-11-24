package auth

import (
	"context"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformwebauthn "github.com/guliangiam/guliangiam/internal/platform/webauthn"
)

// WebAuthnRegistrationBeginCommand starts a WebAuthn registration ceremony.
type WebAuthnRegistrationBeginCommand struct {
	OrganizationSlug string
	Username         string
	// SessionUserID/SessionOrganizationID 是传输层从可信会话注入的身份。
	// 自服务注册只允许为会话本人登记凭据；空值仅出现在登录流程。
	SessionUserID         string
	SessionOrganizationID string
}

// WebAuthnRegistrationBeginResult contains the challenge for the client.
type WebAuthnRegistrationBeginResult struct {
	Challenge        string `json:"challenge"`
	RelyingPartyID   string `json:"rpId"`
	RelyingPartyName string `json:"rpName"`
	UserID           string `json:"userId"`
	ChallengeID      string `json:"challengeId"`
	UserDisplayName  string `json:"userDisplayName"`
	Timeout          int64  `json:"timeout"`
	// ExcludeCredentials 是该用户已有活跃凭据的 base64url credentialId，
	// 浏览器据此在客户端阻止同一认证器重复注册。
	ExcludeCredentials []string `json:"excludeCredentials,omitempty"`
	// UserVerification 与 finish 的 UV 校验同源：required（强制 UV）或 preferred。
	// 只靠 finish 侧拒绝会让用户先完成一次 UP-only 注册、再被服务端打回。
	UserVerification string `json:"userVerification"`
}

// WebAuthnRegistrationFinishCommand completes a WebAuthn registration ceremony.
type WebAuthnRegistrationFinishCommand struct {
	OrganizationSlug string
	Username         string
	// SessionUserID/SessionOrganizationID 是传输层从可信会话注入的身份；
	// 非空时必须与挑战绑定用户一致，防止把攻击者凭据登记到受害者账号。
	SessionUserID         string
	SessionOrganizationID string
	ChallengeID           string
	Challenge             string
	CredentialID          string
	PublicKey             string
	ClientDataJSON        string
	AuthenticatorData     string
	AttestationObject     string
	AttestationType       string
	Transports            []string
	PreviousSessionID     string // S9-sensitive: old session to revoke after WebAuthn credential enrollment
}

// WebAuthnRegistrationFinishResult contains the result of a WebAuthn registration.
type WebAuthnRegistrationFinishResult struct {
	Status       string `json:"status"`
	CredentialID string `json:"credentialId"`
}

// WebAuthnAuthenticationBeginCommand starts a WebAuthn authentication ceremony.
type WebAuthnAuthenticationBeginCommand struct {
	OrganizationSlug string
	Username         string
}

// WebAuthnAuthenticationBeginResult contains the challenge for the client.
type WebAuthnAuthenticationBeginResult struct {
	Challenge            string   `json:"challenge"`
	ChallengeID          string   `json:"challengeId"`
	RelyingPartyID       string   `json:"rpId"`
	AllowedCredentialIDs []string `json:"allowedCredentialIds,omitempty"`
	Timeout              int64    `json:"timeout"`
	// UserVerification 由组织/部署策略决定：required（强制 UV）或 preferred。
	// 浏览器必须拿到这个值才能在弹窗里要求 PIN/生物识别；只靠 finish 侧校验
	// 会让用户先完成一次 UP-only 断言，再被服务端拒绝。
	UserVerification string `json:"userVerification"`
}

// WebAuthnAuthenticationFinishCommand completes a WebAuthn authentication ceremony.
//
// 命令同时承载协议上下文：控制台在发起 ceremony 之前就解析出目标应用与
// OAuth2/OIDC 握手参数，finish 因此能把它们交给 handleLoggedIn，与密码登录
// 共用同一条后处理路径（《开发设计》§45.3）。
type WebAuthnAuthenticationFinishCommand struct {
	OrganizationSlug  string
	Username          string
	ChallengeID       string
	Challenge         string
	CredentialID      string
	Signature         string
	AuthenticatorData string
	ClientDataJSON    string

	// 协议上下文（可为空 = 控制台普通登录，只落会话不产协议产物）。
	ApplicationID string
	ResponseType  string
	RedirectURI   string
	State         string
	Nonce         string
	Scope         []string

	// S9-sensitive: 请求携带的旧会话，在同一事务内撤销（防会话固定）。
	PreviousSessionID string

	// 会话元数据：由传输层从连接与请求头解析，不是客户端可自报的字段
	// （LastSigninIP 与异常登录检测都依赖它）。
	IPAddress string
	UserAgent string
}

const webAuthnChallengeTimeout = 5 * time.Minute

// webAuthnAllowedOrigins 是断言 origin 校验的允许集合：CORS 白名单（跨域部署时配置）
// ∪ ISSUER_URL 的来源（同源部署时白名单通常为空，控制台与 API 同域即 ISSUER_URL 的来源）。
//
// 两条来源都是**服务端配置**，不接受请求头推导：Host / Origin 头由调用方控制，用它们
// 构造期望值等于把闸门交给攻击者。集合为空时返回空切片，校验会 fail-closed。
func (s *Service) webAuthnAllowedOrigins() []string {
	origins := make([]string, 0, len(s.WebAuthnAllowedOrigins)+1)
	origins = append(origins, s.WebAuthnAllowedOrigins...)
	if issuerOrigin := platformwebauthn.OriginOfURL(s.IssuerURL); issuerOrigin != "" {
		origins = append(origins, issuerOrigin)
	}
	// 开发期默认来源与管理员面共用同一函数，避免「管理员能注册、登录不能用」。
	// 非 development 环境原样返回，不注入任何开发来源。
	return platformwebauthn.WithDevelopmentOrigins(origins)
}

// webAuthnRelyingPartyID 是 ceremony 的作用域（RP ID），从同一份来源集合推导：取所有
// 来源主机的公共后缀（同源部署 = 主机本身，控制台与 API 分处不同子域 = 可注册域）。
// 此前 RP ID 取组织 slug（如 acme）——那不是合法域，浏览器会直接拒绝 ceremony
// （《开发设计》31.2 缺陷表登记）。
func (s *Service) webAuthnRelyingPartyID() (string, error) {
	rpID, err := platformwebauthn.RelyingPartyIDFor(s.webAuthnAllowedOrigins())
	if err != nil {
		return "", apperrors.NewInternalf(
			"WebAuthn RP ID unavailable (%s): configure CORS_ALLOWED_ORIGINS / ISSUER_URL so a shared domain can be derived", err.Error())
	}
	return rpID, nil
}

// requireWebAuthnOriginConfig 在来源集合为空时给出可操作的配置错误。
// 空集合会让所有 ceremony 的 origin 校验失败（fail-closed），因此宁可在验签前就把
// 「该配哪个变量」讲清楚。
func (s *Service) requireWebAuthnOriginConfig() error {
	if len(s.webAuthnAllowedOrigins()) == 0 {
		return apperrors.NewInternal("WebAuthn origin allowlist is empty: configure CORS_ALLOWED_ORIGINS or ISSUER_URL")
	}
	return nil
}

// WebAuthnChallengeRetention 是清理作业在「已过期」之后还保留挑战行的时长：
// 过期行不影响正确性（ConsumeChallenge 自带 expires_at > now() 谓词），保留 24h 是为了
// 让「这次 ceremony 为什么失败」在库里有迹可循，之后由周期作业物理删除。
const WebAuthnChallengeRetention = 24 * time.Hour

// CleanupExpiredWebAuthnChallenges 删除过期超过 retention 的挑战行，返回删除条数。
// 由 bootstrap 的周期清理作业调用（与过期会话清理同一节拍），不在请求路径上顺带删。
func (s *Service) CleanupExpiredWebAuthnChallenges(ctx context.Context, retention time.Duration) (int64, error) {
	return s.UOW.WebAuthnCredentials().DeleteExpiredChallenges(ctx, s.Clock.Now().Add(-retention))
}

// WebAuthnRegistrationBegin initiates a WebAuthn credential registration.
