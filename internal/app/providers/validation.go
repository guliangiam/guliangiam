package providers

import (
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	domainnotification "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func validateCreateProviderCommand(command CreateProviderCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if strings.TrimSpace(command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName is required")
	}
	if strings.TrimSpace(command.Category) == "" {
		return apperrors.NewInvalidArgument("category is required")
	}
	if strings.TrimSpace(command.ProviderType) == "" {
		return apperrors.NewInvalidArgument("providerType is required")
	}

	if err := validateOptionalURL(command.ProviderURL, "providerUrl"); err != nil {
		return err
	}
	if err := validateOptionalCallbackURL(command.CallbackUrl); err != nil {
		return err
	}
	_, err := normalizeProviderScopes(command.Scopes)
	return err
}

func validatePatchProviderCommand(command PatchProviderCommand) error {
	if command.DisplayName != nil && strings.TrimSpace(*command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName must not be blank")
	}
	if command.Category != nil && strings.TrimSpace(*command.Category) == "" {
		return apperrors.NewInvalidArgument("category must not be blank")
	}
	if command.ProviderType != nil && strings.TrimSpace(*command.ProviderType) == "" {
		return apperrors.NewInvalidArgument("providerType must not be blank")
	}
	// URL 与 scopes 的取值校验依赖已加载的持久记录，由 validateProviderPatchWrite
	// 在事务内按「本次真正变化的字段」执行；这里只做与存量无关的形态检查。
	return nil
}

// validateProviderPatchWrite 只校验本次 PATCH 真正改变的字段，必须在
// loadProviderTx 之后调用：校验先于加载会让一条存量的非法 providerUrl /
// callbackUrl / scopes 阻断任何无关字段的修改。规则与 applications 的
// validateApplicationReverseProxyWrite 一致：未提交或值未变就原样保留。
func validateProviderPatchWrite(provider *Provider, command PatchProviderCommand) error {
	detail, ok := provider.Detail.(*domain.Provider)
	if !ok {
		return nil
	}
	if command.ProviderURL != nil && strings.TrimSpace(*command.ProviderURL) != strings.TrimSpace(detail.ProviderURL) {
		if err := validateOptionalURL(*command.ProviderURL, "providerUrl"); err != nil {
			return err
		}
	}
	if command.CallbackUrl != nil && strings.TrimSpace(*command.CallbackUrl) != strings.TrimSpace(detail.CallbackUrl) {
		if err := validateOptionalCallbackURL(*command.CallbackUrl); err != nil {
			return err
		}
	}
	if command.Scopes != nil && providerScopesChanged(detail.Scopes, command.Scopes) {
		if _, err := normalizeProviderScopes(command.Scopes); err != nil {
			return err
		}
	}
	return nil
}

// providerScopesChanged 只把完全相同的切片视作「未变化」：存量里带空格、逗号
// 或重复项的旧值原样回传时不再 normalize，避免一次无关 PATCH 静默改写数据。
func providerScopesChanged(current, submitted []string) bool {
	if len(current) != len(submitted) {
		return true
	}
	for index := range current {
		if current[index] != submitted[index] {
			return true
		}
	}
	return false
}

func validateUpdateProviderConfigCommand(command UpdateProviderConfigCommand) error {
	if strings.TrimSpace(command.ProviderID) == "" {
		return apperrors.NewInvalidArgument("providerId is required")
	}
	if strings.TrimSpace(command.ConfigType) == "" {
		return apperrors.NewInvalidArgument("configType is required")
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
			map[string]any{metaKeyField: fieldName, metaKeyValue: trimmed, "format": "uri"},
		)
	}
	if hasUserInfoOrFragment(parsed) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fieldName+" must not contain userinfo or fragment",
			map[string]any{metaKeyField: fieldName, metaKeyValue: trimmed, metaKeyFormat: "uri"},
		)
	}

	return nil
}

// hasUserInfoOrFragment rejects the two shapes that survive a well-formed
// absolute-URL parse but can never be dereferenced safely here: embedded
// credentials (https://user:pw@host/...) and a client-side fragment.
func hasUserInfoOrFragment(parsed *url.URL) bool {
	return parsed.User != nil || parsed.Fragment != "" || parsed.RawFragment != ""
}

// validateOptionalCallbackURL accepts an empty value (providers may resolve the
// redirect URI from providerUrl) and otherwise requires an absolute http(s) URL:
// the callback is dereferenced by the browser during the OAuth round-trip, so a
// relative path or a non-web scheme can never complete the flow.
func validateOptionalCallbackURL(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if utf8.RuneCountInString(trimmed) > maxCallbackURLLength {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"callbackUrl must not exceed 255 characters",
			map[string]any{metaKeyField: "callbackUrl", metaKeyValue: utf8.RuneCountInString(trimmed), metaKeyMax: maxCallbackURLLength},
		)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"callbackUrl must be an absolute http(s) URL",
			map[string]any{metaKeyField: "callbackUrl", metaKeyValue: trimmed, metaKeyFormat: "uri"},
		)
	}
	// The browser dereferences this URL during the OAuth round-trip; embedded
	// credentials would leak into the redirect and a fragment never reaches the
	// server, so both are rejected here (same rule as applications' jwks_uri).
	if hasUserInfoOrFragment(parsed) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"callbackUrl must not contain userinfo or fragment",
			map[string]any{metaKeyField: "callbackUrl", metaKeyValue: trimmed, metaKeyFormat: "uri"},
		)
	}
	return nil
}

// providerScopeTokenPattern is RFC 6749 §3.3's scope-token (NQCHAR): printable
// ASCII except space, double quote and backslash.
var providerScopeTokenPattern = regexp.MustCompile(`^[\x21\x23-\x5B\x5D-\x7E]+$`)

// normalizeProviderScopes accepts the shapes clients actually send — a JSON
// array of tokens, or entries that still carry whitespace/comma separators —
// and returns the tokens that will be stored: trimmed, de-duplicated and in
// first-seen order. Empty input stays empty (scopes are optional). A token that
// cannot be a scope-token, or that exceeds the declared column width, is a 422
// naming the submitted entry.
func normalizeProviderScopes(values []string) ([]string, error) {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for index, raw := range values {
		for _, token := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || unicode.IsSpace(r)
		}) {
			if !providerScopeTokenPattern.MatchString(token) || utf8.RuneCountInString(token) > maxProviderScopeLen {
				return nil, apperrors.NewWithMeta(
					apperrors.CodeUnprocessableEntity,
					"scopes contains an invalid scope token",
					map[string]any{
						metaKeyField:  "scopes",
						metaKeyIndex:  index,
						metaKeyValue:  token,
						metaKeyFormat: "scope-token",
						metaKeyMax:    maxProviderScopeLen,
					},
				)
			}
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			normalized = append(normalized, token)
		}
	}
	if len(normalized) > maxProviderScopes {
		return nil, apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"scopes must not contain more than 64 entries",
			map[string]any{metaKeyField: "scopes", metaKeyValue: len(normalized), metaKeyMax: maxProviderScopes},
		)
	}
	return normalized, nil
}

// The helpers below keep 手册 4.14.1's split: the request parsed and bound, but a
// closed set, a cross-field rule or an activation precondition rejected it —
// 422, with metadata naming the field and the accepted values.

var familySecretTypes = map[domain.Family][]string{
	domain.FamilyIdentity: {secretTypeClientSecret, secretTypeClientSecret2, secretTypePassword, secretTypeAppSecret},
	domain.FamilyAI:       {secretTypeAPIKey},
	domain.FamilyPayment:  {secretTypeConfig},
	domain.FamilyNotification: {
		secretTypeAccessToken,
	},
}

var changeableStatuses = []string{string(domain.StatusActive), string(domain.StatusDisabled)}

func validateIdentityKind(kind domain.Kind) error {
	for _, allowed := range identityKinds {
		if string(kind) == allowed {
			return nil
		}
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"unsupported identity provider kind: "+string(kind),
		map[string]any{metaKeyField: metaKeyKind, metaKeyValue: string(kind), metaKeyAllowed: append([]string{}, identityKinds...)},
	)
}

func validatePaymentKind(kind domain.Kind) error {
	for _, allowed := range paymentKinds {
		if string(kind) == allowed {
			return nil
		}
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"unsupported payment provider kind: "+string(kind),
		map[string]any{metaKeyField: metaKeyKind, metaKeyValue: string(kind), metaKeyAllowed: append([]string{}, paymentKinds...)},
	)
}

func validateNotificationKind(kind domain.Kind) error {
	return domainnotification.ProviderTypes.Validate("kind", string(kind))
}

func validatePaymentEnvironment(environment string, variant domain.Variant) error {
	if environment == string(variant) {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"payment environment must match variant",
		map[string]any{
			metaKeyField: "environment",
			metaKeyValue: environment,
			"expected":   string(variant),
		},
	)
}

func validateActivationReadiness(testPassedAt *time.Time) error {
	if testPassedAt != nil {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"payment provider must pass connection testing before activation",
		map[string]any{metaKeyField: "testPassedAt"},
	)
}

func validateConfigSealed() error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"configSealed must be an envelope-encrypted value",
		map[string]any{metaKeyField: "configSealed", "format": "envelope"},
	)
}

func validateChangeStatus(status domain.Status) error {
	for _, allowed := range changeableStatuses {
		if string(status) == allowed {
			return nil
		}
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"status must be active or disabled",
		map[string]any{
			metaKeyField:   "status",
			metaKeyValue:   string(status),
			metaKeyAllowed: append([]string{}, changeableStatuses...),
		},
	)
}

func validateSecretType(family domain.Family, secretType string) error {
	allowed := familySecretTypes[family]
	for _, value := range allowed {
		if secretType == value {
			return nil
		}
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"unsupported "+string(family)+" secretType: "+secretType,
		map[string]any{metaKeyField: metaKeySecretType, metaKeyValue: secretType, metaKeyAllowed: append([]string{}, allowed...)},
	)
}
