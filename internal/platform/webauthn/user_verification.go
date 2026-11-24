package webauthn

import "strings"

// begin 响应下发给浏览器的 userVerification 取值。浏览器据此决定弹窗是否要求
// PIN / 生物识别；服务端 finish 侧按同一策略校验 flags.UV。
const (
	UserVerificationRequired  = "required"
	UserVerificationPreferred = "preferred"
)

// 组织级 passkey 策略键（organization.Security.MfaPolicy）。缺省即保持历史行为。
const (
	PolicyKeyPasskeyEnabled          = "passkeyEnabled"
	PolicyKeyWebAuthnEnabled         = "webauthnEnabled"
	PolicyKeyRequireUV               = "requireUV"
	PolicyKeyRequireUserVerification = "requireUserVerification"
)

// RequiresUserVerification 合并部署级开关与组织策略。部署开关只能收紧要求，
// 组织可以进一步收紧，但都不能把已强制的 UV 放宽回 preferred。
//
// policy 以原始 map 传入，平台层不依赖 domain 包；自服务注册、管理员代注册与
// 登录断言共用这一处派生，避免各写一份后出现「begin 说 preferred、finish 却要求 UV」。
func RequiresUserVerification(deployRequired bool, policy map[string]any) bool {
	if deployRequired {
		return true
	}
	required, _ := PolicyBool(policy, PolicyKeyRequireUV, PolicyKeyRequireUserVerification)
	return required
}

// UserVerificationValue 把策略裁决映射成 begin 响应里的 userVerification 字符串。
func UserVerificationValue(required bool) string {
	if required {
		return UserVerificationRequired
	}
	return UserVerificationPreferred
}

// PolicyBool 读取第一个命中的策略键，并接受组织策略实际存储的 JSON 形态：
// bool、"true"/"false"、0/1。
func PolicyBool(policy map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		value, ok := policy[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed, true
		case string:
			switch strings.ToLower(strings.TrimSpace(typed)) {
			case "true":
				return true, true
			case "false":
				return false, true
			}
		case int:
			return typed != 0, true
		case int64:
			return typed != 0, true
		case float64:
			return typed != 0, true
		}
	}
	return false, false
}
