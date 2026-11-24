package users

// 用户字段名（DTO profile / CSV 列映射 / 校验标签共用）。
const fieldEmail = "email"

// 批量校验错误元数据的键：前端按 index 定位到具体条目。
const (
	metaKeyIndex  = "index"
	metaKeyField  = "field"
	metaKeyValue  = "value"
	metaKeyFormat = "format"
)

// MFA 方法类型标识。写入 MfaMethods[].MethodType、PreferredMfaType 与
// MfaSetupSummary，同时作为 setup 分派与摘要键。
const (
	mfaMethodApp    = "app"
	mfaMethodTOTP   = "totp"
	mfaMethodEmail  = "email"
	mfaMethodSMS    = "sms"
	mfaMethodRadius = "radius"
	mfaMethodPush   = "push"
)

// allowedMfaSetupTypes is the closed set MfaType accepts in setup flows. The
// dispatcher and the 422 `allowed` metadata both read it, so a value cannot be
// accepted while missing from the metadata.
var allowedMfaSetupTypes = []string{
	mfaMethodApp,
	mfaMethodTOTP,
	mfaMethodEmail,
	mfaMethodSMS,
	mfaMethodRadius,
	mfaMethodPush,
}
