package applications

import (
	"encoding/xml"
	"io"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var applicationSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// clientIDPattern enforces: starts with a letter, followed by letters, digits, hyphens, or underscores.
var clientIDPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// The grant-type closed sets live in grant_types_gen.go, generated from
// contracts/registries/grant-types.yaml. That single registry also drives the
// web console selector and OIDC discovery (OPEN-API-68); do not hand-edit the
// generated slices.
var allowedTokenEndpointAuthMethods = append([]string{}, applicationdomain.TokenEndpointAuthMethods...)

var allowedTokenSigningMethods = []string{
	applicationdomain.TokenSigningMethodRS256,
	applicationdomain.TokenSigningMethodSM2SM3,
}

// A client secret only makes sense with an auth method that actually sends it.
var allowedSecretAuthMethods = []string{"client_secret_basic", "client_secret_post"}

// requiresClientSecretMethod 报告该认证方式是否必须持有 app_client_secrets 行。
// private_key_jwt 的凭据是 JWKS，tls_client_auth 的凭据是证书，none 是无凭据；
// 三者都不应因缺少 secret 被拒绝。
func requiresClientSecretMethod(method string) bool {
	return method == applicationdomain.TokenEndpointAuthMethodClientSecretBasic ||
		method == applicationdomain.TokenEndpointAuthMethodClientSecretPost
}

func validateCreateApplicationCommand(command CreateApplicationCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Slug) == "" {
		return apperrors.NewInvalidArgument("slug is required")
	}
	if err := validateApplicationSlug(command.Slug); err != nil {
		return err
	}
	if strings.TrimSpace(command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName is required")
	}
	if err := validateApplicationTextLength("displayName", command.DisplayName); err != nil {
		return err
	}
	if err := validateApplicationTextLength("description", command.Description); err != nil {
		return err
	}
	if strings.TrimSpace(command.Status) == "" {
		return apperrors.NewInvalidArgument("status is required")
	}
	if err := validateCustomScopes(command.CustomScopes); err != nil {
		return err
	}
	// allowMissingSecretOnCreate=true — NewApplication auto-generates a
	// clientSecret when the auth method is confidential but none is provided.
	if err := validateOAuthClientSettings(command.ClientID, command.ClientSecret, command.TokenEndpointAuthMethod, command.RedirectURIs, command.GrantTypes, true); err != nil {
		return err
	}
	if err := validatePublicClientGrantCombination(command.TokenEndpointAuthMethod, command.ClientSecret, command.GrantTypes); err != nil {
		return err
	}
	if err := validateTokenSigningMethod(command.TokenSigningMethod); err != nil {
		return err
	}

	return validateOptionalURL(command.HomepageURL, "homepageUrl")
}

func validatePatchApplicationCommand(command PatchApplicationCommand) error {
	if command.Slug != nil {
		if err := validateApplicationSlug(*command.Slug); err != nil {
			return err
		}
	}
	if command.DisplayName != nil {
		if strings.TrimSpace(*command.DisplayName) == "" {
			return apperrors.NewInvalidArgument("displayName must not be blank")
		}
		if err := validateApplicationTextLength("displayName", *command.DisplayName); err != nil {
			return err
		}
	}
	if command.Description != nil {
		if err := validateApplicationTextLength("description", *command.Description); err != nil {
			return err
		}
	}
	if command.RedirectURIs != nil {
		if err := validateRedirectURILimits("redirectUris", normalizeStringList(*command.RedirectURIs)); err != nil {
			return err
		}
	}
	if command.Status != nil && strings.TrimSpace(*command.Status) == "" {
		return apperrors.NewInvalidArgument("status must not be blank")
	}
	if command.HomepageURL != nil {
		if err := validateOptionalURL(*command.HomepageURL, "homepageUrl"); err != nil {
			return err
		}
	}
	if command.CustomScopes != nil {
		if err := validateCustomScopes(*command.CustomScopes); err != nil {
			return err
		}
	}
	if command.TokenSigningMethod != nil {
		if err := validateTokenSigningMethod(*command.TokenSigningMethod); err != nil {
			return err
		}
	}
	return nil
}

// normalizeTokenSigningMethod maps "unset/blank" onto the schema default.
// token_signing_method is NOT NULL DEFAULT 'RS256', so an empty string is not
// "unset" at the storage layer: it is stored as an empty value and shadows the
// default.
func normalizeTokenSigningMethod(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return applicationdomain.TokenSigningMethodRS256
	}
	return trimmed
}

func validateTokenSigningMethod(value string) error {
	normalized := normalizeTokenSigningMethod(value)
	for _, allowed := range allowedTokenSigningMethods {
		if normalized == allowed {
			return nil
		}
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"unsupported tokenSigningMethod: "+normalized,
		map[string]any{
			"field":   "tokenSigningMethod",
			"value":   normalized,
			"allowed": append([]string{}, allowedTokenSigningMethods...),
		},
	)
}

func validateApplicationSlug(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return apperrors.NewInvalidArgument("slug must not be blank")
	}
	if !applicationSlugPattern.MatchString(trimmed) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"slug must contain only lowercase letters, digits, and hyphens",
			map[string]any{metaKeyField: "slug", metaKeyValue: trimmed, metaKeyFormat: "slug"},
		)
	}

	return nil
}

func validateCustomScopes(scopes []ApplicationCustomScopeCommand) error {
	seen := make(map[string]struct{}, len(scopes))
	for index, scope := range scopes {
		name := strings.TrimSpace(scope.Scope)
		if name == "" {
			return apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "customScopes scope is required", map[string]any{metaKeyIndex: index})
		}
		if _, ok := seen[name]; ok {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "customScopes scope must be unique", map[string]any{metaKeyIndex: index, "scope": name})
		}
		seen[name] = struct{}{}
	}
	return nil
}

// validateOAuthClientSettings checks OIDC/OAuth2 client configuration fields.
// allowMissingSecretOnCreate controls whether a confidential auth method may
// omit clientSecret — on create, NewApplication auto-generates one; on
// update, missingSecret is only allowed when the existing record already
// has a secret hash (the empty string means "keep current").
func validateOAuthClientSettings(clientID, clientSecret, tokenEndpointAuthMethod string, redirectURIs, grantTypes []string, allowMissingSecretOnCreate bool) error {
	normalizedClientID := strings.TrimSpace(clientID)
	normalizedClientSecret := strings.TrimSpace(clientSecret)
	rawAuthMethod := strings.ToLower(strings.TrimSpace(tokenEndpointAuthMethod))
	if rawAuthMethod != "" && !slices.Contains(allowedTokenEndpointAuthMethods, rawAuthMethod) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"tokenEndpointAuthMethod contains unsupported value",
			map[string]any{
				metaKeyField: "tokenEndpointAuthMethod",
				metaKeyValue: rawAuthMethod,
				"allowed":    append([]string{}, allowedTokenEndpointAuthMethods...),
			},
		)
	}
	// client_secret_jwt 需要原始 secret 参与 HS256 验签。仓库的 encryption_keys
	// 只有只读元数据仓储，没有密钥解析/写入服务，无法完成信封加密往返；按拍板
	// 结论降级为“不支持、不对外声明”，因此它不在注册闭集内，会落到上面的
	// unsupported 分支（422）；DCR 另在 validateDCRRequest 返回 400
	// invalid_client_metadata。
	normalizedAuthMethod := normalizeTokenEndpointAuthMethod(rawAuthMethod, normalizedClientSecret)
	normalizedRedirectURIs := normalizeStringList(redirectURIs)
	normalizedGrantTypes := normalizeStringList(grantTypes)
	if err := validateRedirectURILimits("redirectUris", normalizedRedirectURIs); err != nil {
		return err
	}

	// ClientID/ClientSecret are auto-generated by the domain factory when empty.
	// Only validate format if explicitly supplied by the caller (DCR use case).
	if normalizedClientID != "" && !clientIDPattern.MatchString(normalizedClientID) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"clientId must start with a letter and contain only letters, digits, hyphens, or underscores",
			map[string]any{metaKeyField: "clientId", metaKeyValue: normalizedClientID, metaKeyFormat: "client_id"},
		)
	}
	// A caller-supplied secret must clear a minimum entropy floor. Empty means
	// "auto-generate" (create) or "keep current" (update) and is handled below.
	// Never echo the secret itself in the error metadata.
	if normalizedClientSecret != "" && utf8.RuneCountInString(normalizedClientSecret) < minClientSecretLength {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"clientSecret must be at least "+strconv.Itoa(minClientSecretLength)+" characters",
			map[string]any{metaKeyField: "clientSecret", metaKeyMin: minClientSecretLength},
		)
	}
	if normalizedAuthMethod == applicationdomain.TokenEndpointAuthMethodNone && normalizedClientSecret != "" {
		// Never echo the secret itself; name the field that has to change instead.
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"clientSecret requires a secret-based tokenEndpointAuthMethod",
			map[string]any{
				metaKeyField: "tokenEndpointAuthMethod",
				metaKeyValue: rawAuthMethod,
				"allowed":    append([]string{}, allowedSecretAuthMethods...),
			},
		)
	}
	if requiresClientSecretMethod(normalizedAuthMethod) && normalizedClientSecret == "" && !allowMissingSecretOnCreate {
		return apperrors.NewInvalidArgument("clientSecret is required when tokenEndpointAuthMethod requires client authentication")
	}
	for index, uri := range normalizedRedirectURIs {
		if err := validateOptionalURL(uri, "redirectUris"); err != nil {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "redirectUris must contain valid absolute URLs", map[string]any{metaKeyIndex: index, metaKeyValue: uri})
		}
		// Registration is where a wildcard has to die. `https://*.example.com/cb`
		// passes the absolute-URL check above (url.Parse accepts the `*`), and the
		// authorize-time matcher would never admit it — so it used to be a silently
		// dead registration that only became a security question once a second,
		// more permissive matcher existed (设计报告 14.6.3).
		if err := applicationdomain.ValidateRedirectURI(uri); err != nil {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "redirectUris contains an unusable value", map[string]any{metaKeyIndex: index, metaKeyValue: uri})
		}
	}

	for index, grantType := range normalizedGrantTypes {
		if !slices.Contains(allowedClientGrantTypes, grantType) {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"grantTypes contains unsupported value",
				map[string]any{
					metaKeyIndex: index,
					"grantType":  grantType,
					metaKeyField: "grantTypes",
					metaKeyValue: grantType,
					"allowed":    append([]string{}, allowedClientGrantTypes...),
				},
			)
		}
	}

	return nil
}

// validatePublicClientGrantCombination keeps a public client (none) to grants
// the token endpoint can actually redeem. authorization_code is the only
// interactive grant implemented today, and client_credentials requires a
// confidential client — both are rejected at the token endpoint, so accepting
// them here would persist a configuration that can never authenticate.
//
// The method is normalized with the same rule as creation, because an explicit
// empty method with no secret resolves to none.
func validatePublicClientGrantCombination(tokenEndpointAuthMethod, clientSecret string, grantTypes []string) error {
	rawMethod := strings.ToLower(strings.TrimSpace(tokenEndpointAuthMethod))
	normalizedMethod := normalizeTokenEndpointAuthMethod(rawMethod, strings.TrimSpace(clientSecret))
	if normalizedMethod != applicationdomain.TokenEndpointAuthMethodNone {
		return nil
	}
	normalizedGrantTypes := normalizeStringList(grantTypes)
	if !slices.Contains(normalizedGrantTypes, grantTypeAuthorizationCode) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"tokenEndpointAuthMethod none requires the authorization_code grant",
			map[string]any{
				metaKeyField: "grantTypes",
				metaKeyValue: append([]string{}, normalizedGrantTypes...),
				"expected":   grantTypeAuthorizationCode,
			},
		)
	}
	if slices.Contains(normalizedGrantTypes, grantTypeClientCredentials) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"tokenEndpointAuthMethod none cannot be combined with the client_credentials grant",
			map[string]any{
				metaKeyField: "grantTypes",
				metaKeyValue: grantTypeClientCredentials,
			},
		)
	}
	return nil
}

// normalizeImportTokenEndpointAuthMethod resolves the auth method carried by an
// export document. An empty value is preserved: legacy rows could leave the
// column empty, and the token endpoint already resolves it to none when no
// secret exists.
//
// The export DTO deliberately excludes secrets, JWKS and certificates. A method
// that needs any of those would be imported as a client that can never
// authenticate, so it is rejected here with an identifiable 422 instead of
// landing in the repository.
func normalizeImportTokenEndpointAuthMethod(rawMethod string) (string, error) {
	method := strings.ToLower(strings.TrimSpace(rawMethod))
	if method == "" {
		return "", nil
	}
	if !slices.Contains(allowedTokenEndpointAuthMethods, method) {
		return "", apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"tokenEndpointAuthMethod contains unsupported value",
			map[string]any{
				metaKeyField: "tokenEndpointAuthMethod",
				metaKeyValue: method,
				"allowed":    append([]string{}, allowedTokenEndpointAuthMethods...),
			},
		)
	}
	if method == applicationdomain.TokenEndpointAuthMethodPrivateKeyJWT || isMTLSMethod(method) {
		return "", apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"tokenEndpointAuthMethod requires client material that the export does not carry; register it through the console",
			map[string]any{
				metaKeyField: "tokenEndpointAuthMethod",
				metaKeyValue: method,
				"expected":   "client_material_via_console",
			},
		)
	}
	return method, nil
}

func validateReplaceProviderBindingCommands(commands []ReplaceApplicationProviderBindingCommand) error {
	seenProviderIDs := map[string]struct{}{}

	for index, command := range commands {
		providerID := strings.TrimSpace(command.ProviderID)
		if providerID == "" {
			return apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "provider binding providerId is required", map[string]any{metaKeyIndex: index})
		}

		if _, ok := seenProviderIDs[providerID]; ok {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "provider binding providerId must be unique", map[string]any{metaKeyIndex: index, "providerId": providerID})
		}
		seenProviderIDs[providerID] = struct{}{}
	}

	return nil
}

// validateApplicationTextLength enforces the same-family VARCHAR(255) ceiling
// for displayName / description. The count is runes, not bytes, so it matches
// how a VARCHAR(n) column counts characters.
func validateApplicationTextLength(field, value string) error {
	length := utf8.RuneCountInString(value)
	if length <= maxApplicationTextLength {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		field+" must not exceed "+strconv.Itoa(maxApplicationTextLength)+" characters",
		map[string]any{metaKeyField: field, metaKeyValue: length, metaKeyMax: maxApplicationTextLength},
	)
}

// validateRedirectURILimits keeps the write inside the declared storage shape:
// at most 64 entries (the request-slice ceiling) and at most 255 characters per
// entry (app_oauth_config.redirect_uris VARCHAR(255)[]).
//
// field is the name the caller's payload actually uses — the console speaks
// camelCase, RFC 7591 speaks snake_case — so the 422 names a field the caller
// sent instead of one it has to translate. Lists must already be normalized:
// the entry count and length are the ones that will be written.
func validateRedirectURILimits(field string, redirectURIs []string) error {
	if len(redirectURIs) > maxRedirectURIs {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			field+" must not contain more than "+strconv.Itoa(maxRedirectURIs)+" entries",
			map[string]any{metaKeyField: field, metaKeyValue: len(redirectURIs), metaKeyMax: maxRedirectURIs},
		)
	}
	for index, uri := range redirectURIs {
		length := utf8.RuneCountInString(strings.TrimSpace(uri))
		if length <= maxRedirectURIEntryLength {
			continue
		}
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			field+" entries must not exceed "+strconv.Itoa(maxRedirectURIEntryLength)+" characters",
			map[string]any{
				metaKeyIndex: index,
				metaKeyField: field,
				metaKeyValue: length,
				metaKeyMax:   maxRedirectURIEntryLength,
			},
		)
	}
	return nil
}

func validateOptionalURL(value, fieldName string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fieldName+" must be a valid absolute URL",
			map[string]any{metaKeyField: fieldName, metaKeyValue: trimmed, metaKeyFormat: "uri"},
		)
	}

	return nil
}

// validateClientJWKSURI accepts only absolute HTTPS URLs without userinfo or
// fragment. SSRF (DNS/IP allowlist) is enforced by the fetcher at request time;
// this is the shape check at the registration boundary (OPEN-API-61).
func validateClientJWKSURI(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) > 2048 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"jwks_uri is too long", map[string]any{metaKeyField: "jwks_uri", "max": 2048})
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"jwks_uri must be an absolute HTTPS URL without userinfo or fragment",
			map[string]any{metaKeyField: "jwks_uri"})
	}
	return nil
}

// samlMetadataNamespace is the namespace SAML 2.0 requires on a metadata root
// element. Empty is tolerated because some IdPs emit namespace-less metadata and
// the SP login path parses it namespace-agnostically.
const samlMetadataNamespace = "urn:oasis:names:tc:SAML:2.0:metadata"

// validateSamlMetadata accepts an empty value (metadata may come from
// samlMetadataUrl instead) and otherwise requires a well-formed XML document
// whose root is a SAML EntityDescriptor or EntitiesDescriptor.
func validateSamlMetadata(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	decoder := xml.NewDecoder(strings.NewReader(trimmed))
	rootSeen := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"samlMetadata must be well-formed XML",
				map[string]any{metaKeyField: "samlMetadata", metaKeyFormat: "xml"},
			)
		}
		start, ok := token.(xml.StartElement)
		if !ok || rootSeen {
			continue
		}
		rootSeen = true
		if start.Name.Local != "EntityDescriptor" && start.Name.Local != "EntitiesDescriptor" {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"samlMetadata root element must be EntityDescriptor or EntitiesDescriptor",
				map[string]any{
					metaKeyField: "samlMetadata",
					metaKeyValue: start.Name.Local,
					"allowed":    []string{"EntityDescriptor", "EntitiesDescriptor"},
				},
			)
		}
		if start.Name.Space != "" && start.Name.Space != samlMetadataNamespace {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"samlMetadata root element must use the SAML metadata namespace",
				map[string]any{metaKeyField: "samlMetadata", metaKeyValue: start.Name.Space, "allowed": []string{samlMetadataNamespace}},
			)
		}
	}
	if !rootSeen {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"samlMetadata must contain a SAML metadata root element",
			map[string]any{metaKeyField: "samlMetadata"},
		)
	}
	return nil
}

// validateSamlMetadataURL accepts an empty value and otherwise requires an
// absolute http(s) URL: the SP login path fetches this document server-side.
func validateSamlMetadataURL(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	length := utf8.RuneCountInString(trimmed)
	if length > maxSamlMetadataURLLength {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"samlMetadataUrl must not exceed "+strconv.Itoa(maxSamlMetadataURLLength)+" characters",
			map[string]any{metaKeyField: "samlMetadataUrl", metaKeyValue: length, metaKeyMax: maxSamlMetadataURLLength},
		)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"samlMetadataUrl must be an absolute http(s) URL",
			map[string]any{metaKeyField: "samlMetadataUrl", metaKeyValue: trimmed, metaKeyFormat: "uri"},
		)
	}
	return nil
}

// validateApplicationSamlWrite checks only the SAML values this request changes.
// Read paths stay untouched: rows written before the rule existed may hold
// values that would not pass today, and resubmitting such a value unchanged must
// keep succeeding instead of turning a legacy row into an unsaveable one.
func validateApplicationSamlWrite(current *applicationdomain.Application, command PatchApplicationCommand) error {
	if command.SamlMetadata != nil && strings.TrimSpace(*command.SamlMetadata) != strings.TrimSpace(current.SamlMetadata) {
		if err := validateSamlMetadata(*command.SamlMetadata); err != nil {
			return err
		}
	}
	if command.SamlMetadataUrl != nil && strings.TrimSpace(*command.SamlMetadataUrl) != strings.TrimSpace(current.SamlMetadataUrl) {
		if err := validateSamlMetadataURL(*command.SamlMetadataUrl); err != nil {
			return err
		}
	}
	return nil
}

// ─── Reverse proxy fields ──────────────────────────────────────────────────

// upstream_host 落库列是 VARCHAR(255)，只接受主机名/IP 加可选端口。
var upstreamHostNamePattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?$`)

// 端口单独用数字模式收敛，避免 strconv 接受 "+80"/"-0" 这类带符号写法。
var upstreamHostPortPattern = regexp.MustCompile(`^[0-9]{1,5}$`)

// ssl_cert 是 SSL 终止证书的标识（控制台提示为 Certificate ID），列宽 VARCHAR(255)；
// 这里只做标识符字符集校验，不引入 PEM 解析，因为该字段不是证书本体。
var sslCertIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:/-]+$`)

// validateApplicationReverseProxyWrite 只校验本次 PATCH 真正改变的反向代理字段：
// 空值一律放行（清除配置），历史行原样回传也不拦截，避免旧数据把整条记录变成不可保存
// （与 validateApplicationSamlWrite 同一规则）。
func validateApplicationReverseProxyWrite(current *applicationdomain.Application, command PatchApplicationCommand) error {
	if command.UpstreamHost != nil && strings.TrimSpace(*command.UpstreamHost) != strings.TrimSpace(current.UpstreamHost) {
		if err := validateUpstreamHost(*command.UpstreamHost); err != nil {
			return err
		}
	}
	if command.SslCert != nil && strings.TrimSpace(*command.SslCert) != strings.TrimSpace(current.SslCert) {
		if err := validateSslCert(*command.SslCert); err != nil {
			return err
		}
	}
	if command.SslMode != nil && strings.TrimSpace(*command.SslMode) != strings.TrimSpace(current.SslMode) {
		if err := validateSslMode(*command.SslMode); err != nil {
			return err
		}
	}
	if command.OtherDomains != nil {
		// 比较前先按写入路径同样的规则（去空白、去重）归一，避免仅空白/重复项差异就触发重校验。
		if !slices.Equal(normalizeStringList(*command.OtherDomains), normalizeStringList(current.OtherDomains)) {
			if err := validateOtherDomains(*command.OtherDomains); err != nil {
				return err
			}
		}
	}
	return validateReverseProxyConsistency(current, command)
}

// sslMode 的目标集合（与前端 Select 一致）。历史 Casdoor 词汇 Auto/Enable/Disable
// 只作为写入别名与语义映射存在，读取与存储都不强制改写。
const (
	sslModeNone         = ""
	sslModeHTTP         = "HTTP"
	sslModeHTTPSAndHTTP = "HTTPS and HTTP"
	sslModeHTTPSOnly    = "HTTPS Only"
)

var allowedSslModes = []string{sslModeNone, sslModeHTTP, sslModeHTTPSAndHTTP, sslModeHTTPSOnly}

// legacySslModeAliases 把历史值映射到语义等价的目标值。
var legacySslModeAliases = map[string]string{
	"Auto":    sslModeNone,
	"Enable":  sslModeHTTPSOnly,
	"Disable": sslModeHTTP,
}

// canonicalSslMode 返回 sslMode 的语义值；ok=false 表示既不是目标值也不是已知别名。
func canonicalSslMode(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if slices.Contains(allowedSslModes, trimmed) {
		return trimmed, true
	}
	canonical, ok := legacySslModeAliases[trimmed]
	return canonical, ok
}

// validateSslMode 只在值变化时被调用：目标四值与历史别名放行，其它值拒绝。
func validateSslMode(value string) error {
	trimmed := strings.TrimSpace(value)
	length := utf8.RuneCountInString(trimmed)
	if length > maxApplicationTextLength {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"sslMode must not exceed "+strconv.Itoa(maxApplicationTextLength)+" characters",
			map[string]any{metaKeyField: "sslMode", metaKeyValue: length, metaKeyMax: maxApplicationTextLength},
		)
	}
	if _, ok := canonicalSslMode(trimmed); ok {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"sslMode must be one of: empty, HTTP, HTTPS and HTTP, HTTPS Only",
		map[string]any{metaKeyField: "sslMode", metaKeyValue: trimmed, metaKeyAllowed: allowedSslModes},
	)
}

// validateReverseProxyConsistency 校验 sslMode / sslCert / upstreamHost / domain /
// otherDomains 之间「单字段合法、组合起来不成立」的关系（R1–R4）。
//
// 每条规则只在该规则涉及的字段相对持久值真正变化时生效（R6）：前端整表提交，
// 若按「字段出现即校验」，历史非法组合会因无关字段的保存被永久拦死。
func validateReverseProxyConsistency(current *applicationdomain.Application, command PatchApplicationCommand) error {
	sslModeChanged := command.SslMode != nil && strings.TrimSpace(*command.SslMode) != strings.TrimSpace(current.SslMode)
	sslCertChanged := command.SslCert != nil && strings.TrimSpace(*command.SslCert) != strings.TrimSpace(current.SslCert)
	upstreamHostChanged := command.UpstreamHost != nil && strings.TrimSpace(*command.UpstreamHost) != strings.TrimSpace(current.UpstreamHost)
	domainChanged := command.Domain != nil && strings.TrimSpace(*command.Domain) != strings.TrimSpace(current.Domain)
	otherDomainsChanged := command.OtherDomains != nil &&
		!slices.Equal(normalizeStringList(*command.OtherDomains), normalizeStringList(current.OtherDomains))

	sslMode := current.SslMode
	if command.SslMode != nil {
		sslMode = *command.SslMode
	}
	sslCert := current.SslCert
	if command.SslCert != nil {
		sslCert = *command.SslCert
	}
	upstreamHost := current.UpstreamHost
	if command.UpstreamHost != nil {
		upstreamHost = *command.UpstreamHost
	}
	domain := current.Domain
	if command.Domain != nil {
		domain = *command.Domain
	}
	otherDomains := current.OtherDomains
	if command.OtherDomains != nil {
		otherDomains = normalizeStringList(*command.OtherDomains)
	}

	sslMode = strings.TrimSpace(sslMode)
	sslCert = strings.TrimSpace(sslCert)
	upstreamHost = strings.TrimSpace(upstreamHost)
	domain = strings.TrimSpace(domain)

	// R1：声明 HTTPS 终止必须有证书；带证书的模式必须真的使用 HTTPS。
	if sslModeChanged || sslCertChanged {
		canonicalMode, modeKnown := canonicalSslMode(sslMode)
		usesCertificate := modeKnown && (canonicalMode == sslModeHTTPSOnly || canonicalMode == sslModeHTTPSAndHTTP)
		if usesCertificate && sslCert == "" {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"sslCert is required when sslMode is HTTPS Only or HTTPS and HTTP",
				map[string]any{metaKeyField: "sslCert", metaKeyRule: "R1", metaKeyValue: sslMode},
			)
		}
		// 未知的历史值无法判定是否使用证书，交给 R6 的「未变化即放行」兜底。
		if modeKnown && !usesCertificate && sslCert != "" {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"sslCert requires sslMode HTTPS Only or HTTPS and HTTP",
				map[string]any{metaKeyField: "sslMode", metaKeyRule: "R1", metaKeyValue: sslCert},
			)
		}
	}

	// R2：接管 SSL（模式或证书非空）就必须有上游。
	if sslModeChanged || sslCertChanged || upstreamHostChanged {
		if (sslMode != "" || sslCert != "") && upstreamHost == "" {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"upstreamHost is required when sslMode or sslCert is set",
				map[string]any{metaKeyField: "upstreamHost", metaKeyRule: "R2"},
			)
		}
	}

	// R3：domain 必须是裸 host；otherDomains 不得包含主域名或在内部重复。
	if domainChanged && domain != "" && !isBareDomainHost(domain) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"domain must be a bare host without scheme, path or whitespace",
			map[string]any{metaKeyField: "domain", metaKeyRule: "R3", metaKeyValue: domain, metaKeyFormat: "host"},
		)
	}
	if domainChanged || otherDomainsChanged {
		primaryHost := applicationdomain.NormalizeReverseProxyHost(domain)
		seenHosts := make(map[string]int, len(otherDomains))
		for index, raw := range otherDomains {
			host := applicationdomain.NormalizeReverseProxyHost(raw)
			if host == "" {
				continue
			}
			if primaryHost != "" && host == primaryHost {
				return apperrors.NewWithMeta(
					apperrors.CodeUnprocessableEntity,
					"otherDomains must not contain the application domain",
					map[string]any{metaKeyField: "otherDomains", metaKeyRule: "R3", metaKeyIndex: index, metaKeyValue: host},
				)
			}
			if previous, ok := seenHosts[host]; ok {
				return apperrors.NewWithMeta(
					apperrors.CodeUnprocessableEntity,
					"otherDomains must not repeat the same host",
					map[string]any{
						metaKeyField: "otherDomains", metaKeyRule: "R3", metaKeyIndex: index,
						metaKeyValue: host, metaKeyDuplicateOf: previous,
					},
				)
			}
			seenHosts[host] = index
		}
	}

	// R4：上游指向本应用自己的 domain / otherDomains（端口不改变「指向自己」的语义）。
	if upstreamHostChanged || domainChanged || otherDomainsChanged {
		upstreamHostOnly := applicationdomain.ReverseProxyHostOnly(applicationdomain.NormalizeReverseProxyHost(upstreamHost))
		if upstreamHostOnly != "" {
			selfHosts := make(map[string]struct{}, len(otherDomains)+1)
			if host := applicationdomain.ReverseProxyHostOnly(applicationdomain.NormalizeReverseProxyHost(domain)); host != "" {
				selfHosts[host] = struct{}{}
			}
			for _, raw := range otherDomains {
				if host := applicationdomain.ReverseProxyHostOnly(applicationdomain.NormalizeReverseProxyHost(raw)); host != "" {
					selfHosts[host] = struct{}{}
				}
			}
			if _, ok := selfHosts[upstreamHostOnly]; ok {
				return apperrors.NewWithMeta(
					apperrors.CodeUnprocessableEntity,
					"upstreamHost must not point at the application's own domain",
					map[string]any{metaKeyField: "upstreamHost", metaKeyRule: "R4", metaKeyValue: upstreamHostOnly, metaKeyFormat: "host"},
				)
			}
		}
	}
	return nil
}

// isBareDomainHost 判断 domain 是否为裸 host（不允许 scheme、path、端口或空白）。
func isBareDomainHost(value string) bool {
	if strings.ContainsAny(value, "/?# \t\r\n") {
		return false
	}
	if strings.HasPrefix(value, "[") {
		end := strings.LastIndex(value, "]")
		return end == len(value)-1 && net.ParseIP(value[1:end]) != nil
	}
	if net.ParseIP(value) != nil {
		return true
	}
	return upstreamHostNamePattern.MatchString(value)
}

// validateUpstreamHost 接受空值，非空时只接受 `host` 或 `host:port`：
// 主机名/IP + 可选端口（1-65535），带 scheme、空白或其它分隔符的值一律拒绝。
func validateUpstreamHost(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if utf8.RuneCountInString(trimmed) > maxApplicationTextLength {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"upstreamHost must not exceed "+strconv.Itoa(maxApplicationTextLength)+" characters",
			map[string]any{metaKeyField: "upstreamHost", metaKeyValue: utf8.RuneCountInString(trimmed), metaKeyMax: maxApplicationTextLength},
		)
	}
	if !isValidUpstreamHost(trimmed) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"upstreamHost must be a hostname or IP with an optional port (1-65535)",
			map[string]any{metaKeyField: "upstreamHost", metaKeyValue: trimmed, metaKeyFormat: "host_port"},
		)
	}
	return nil
}

func isValidUpstreamHost(value string) bool {
	if strings.Contains(value, "://") {
		return false
	}
	// 带端口的 IPv6 必须写成 [addr]:port；单独 [addr] 也接受。
	if strings.HasPrefix(value, "[") {
		end := strings.LastIndex(value, "]")
		if end < 0 || net.ParseIP(value[1:end]) == nil {
			return false
		}
		rest := value[end+1:]
		if rest == "" {
			return true
		}
		return strings.HasPrefix(rest, ":") && isValidUpstreamPort(rest[1:])
	}
	// 裸 IPv4/IPv6（含未加方括号的 IPv6 字面量）。
	if net.ParseIP(value) != nil {
		return true
	}
	host := value
	if index := strings.LastIndex(value, ":"); index >= 0 {
		host = value[:index]
		if !isValidUpstreamPort(value[index+1:]) {
			return false
		}
	}
	return upstreamHostNamePattern.MatchString(host)
}

func isValidUpstreamPort(value string) bool {
	if !upstreamHostPortPattern.MatchString(value) {
		return false
	}
	port, err := strconv.Atoi(value)
	return err == nil && port >= 1 && port <= 65535
}

// validateOtherDomains 接受空值/空元素，非空元素必须是带 host 的 http(s) 绝对 URL。
func validateOtherDomains(values []string) error {
	for index, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if utf8.RuneCountInString(trimmed) > maxRedirectURIEntryLength {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"otherDomains entries must not exceed "+strconv.Itoa(maxRedirectURIEntryLength)+" characters",
				map[string]any{
					metaKeyIndex: index,
					metaKeyField: "otherDomains",
					metaKeyValue: utf8.RuneCountInString(trimmed),
					metaKeyMax:   maxRedirectURIEntryLength,
				},
			)
		}
		parsed, err := url.Parse(trimmed)
		// userinfo 与 fragment 不是回调地址的一部分：前者会随请求泄露凭据，
		// 后者永远不会到达服务端。与 validateClientJWKSURI 保持同一拒绝口径。
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.User != nil || parsed.Fragment != "" {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"otherDomains must contain absolute http(s) URLs with a host and without userinfo or fragment",
				map[string]any{
					metaKeyIndex:  index,
					metaKeyField:  "otherDomains",
					metaKeyValue:  trimmed,
					metaKeyFormat: "uri",
				},
			)
		}
	}
	return nil
}

// validateSslCert 接受空值，非空时按证书标识的既有约定做长度与字符集校验
// （ssl_cert 是 VARCHAR(255) 的自由引用，本仓库不做证书存在性校验）。
func validateSslCert(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	length := utf8.RuneCountInString(trimmed)
	if length > maxApplicationTextLength {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"sslCert must not exceed "+strconv.Itoa(maxApplicationTextLength)+" characters",
			map[string]any{metaKeyField: "sslCert", metaKeyValue: length, metaKeyMax: maxApplicationTextLength},
		)
	}
	if !sslCertIDPattern.MatchString(trimmed) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"sslCert must be a certificate identifier without whitespace or control characters",
			map[string]any{metaKeyField: "sslCert", metaKeyValue: trimmed, metaKeyFormat: "certificate_id"},
		)
	}
	return nil
}
