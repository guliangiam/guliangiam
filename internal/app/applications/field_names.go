package applications

// OAuth grant 类型：DCR 默认授予与注册校验白名单共用。
const (
	grantTypeAuthorizationCode = "authorization_code"
	grantTypeRefreshToken      = "refresh_token"
	grantTypeClientCredentials = "client_credentials"
)

// 批量校验错误元数据的键。
const (
	metaKeyIndex       = "index"
	metaKeyField       = "field"
	metaKeyValue       = "value"
	metaKeyFormat      = "format"
	metaKeyMax         = "max"
	metaKeyMin         = "min"
	metaKeyAllowed     = "allowed"
	metaKeyRule        = "rule"
	metaKeyDuplicateOf = "duplicateOf"
	metaKeyHost        = "host"
	metaKeyConflictID  = "conflictApplicationId"
	metaKeyConflictKey = "conflictApplicationSlug"
)

// Length ceilings. redirect_uris is VARCHAR(255)[] in the schema, so 255 is the
// declared per-entry ceiling; the array itself has no declared cardinality, so
// 64 follows the existing request-slice ceiling used by the auth surface
// (internal/transport/http/api/v1/auth/request.go).
//
// applications.display_name / description are TEXT, which declares no ceiling.
// The same-family declared length is used instead of inventing one: 20 of 27
// display_name columns and 6 of 10 description columns in contracts/schema are
// VARCHAR(255), and every sibling text column in the applications table
// (category / app_type / title / logo / favicon) is VARCHAR(255).
const (
	maxRedirectURIs           = 64
	maxRedirectURIEntryLength = 255
	maxApplicationTextLength  = 255
	// minClientSecretLength is the floor for caller-supplied client secrets.
	// Auto-generated secrets (gl_cs_ + 43 chars) are far above it; the floor
	// only rejects weak custom values such as "secret" or "123456".
	minClientSecretLength = 16
	// app_saml_config.saml_metadata is TEXT (multi-KB documents), so it has no
	// ceiling; saml_metadata_url is VARCHAR(2048), and that column width is the
	// URL ceiling.
	maxSamlMetadataURLLength = 2048
)
