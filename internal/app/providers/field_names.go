package providers

// Provider secret 类型。RotateProviderSecretCommand.SecretType、native 同步
// 逻辑与 API 的 secretType 字段共用这组取值。
const (
	secretTypeAPIKey       = "api_key"
	secretTypeAccessToken  = "access_token"
	secretTypeClientSecret = "client_secret"
	secretTypeConfig       = "config"
	// identity providers may rotate any of the four secret slots below.
	secretTypeClientSecret2 = "client_secret2"
	secretTypePassword      = "password"
	secretTypeAppSecret     = "app_secret"
)

// 审计 details 里复用的元数据键。
const (
	metaKeyStatus     = "status"
	metaKeyRevision   = "revision"
	metaKeyConfig     = "config"
	metaKeyAllowed    = "allowed"
	metaKeyField      = "field"
	metaKeyKind       = "kind"
	metaKeySecretType = "secretType"
	metaKeyValue      = "value"
	metaKeyIndex      = "index"
	metaKeyFormat     = "format"
	metaKeyMax        = "max"
)

// Write-side ceilings. providers.callback_url is VARCHAR(255) and
// providers.scopes is VARCHAR(255)[] in contracts/schema/20_identity.yaml, so
// 255 is the declared per-entry ceiling; the array declares no cardinality, so
// 64 follows the request-slice ceiling already used by the auth surface
// (internal/transport/http/api/v1/auth/request.go).
const (
	maxCallbackURLLength = 255
	maxProviderScopes    = 64
	maxProviderScopeLen  = 255
)
