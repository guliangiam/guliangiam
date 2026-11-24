package organizations

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformi18n "github.com/guliangiam/guliangiam/internal/platform/i18n"
	"github.com/guliangiam/guliangiam/internal/platform/phonefmt"
)

func validateCreateOrganizationCommand(command CreateOrganizationCommand) error {
	if strings.TrimSpace(command.Slug) == "" {
		return apperrors.NewInvalidArgument("slug is required")
	}
	if strings.TrimSpace(command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName is required")
	}
	if strings.TrimSpace(command.Status) == "" {
		return apperrors.NewInvalidArgument("status is required")
	}
	if err := validateOptionalURL(command.WebsiteURL, "websiteUrl"); err != nil {
		return err
	}
	if err := validateOptionalURL(command.LogoURL, "logoUrl"); err != nil {
		return err
	}
	if err := validateOptionalURL(command.LogoDarkURL, "logoDarkUrl"); err != nil {
		return err
	}
	if err := validateOptionalURL(command.FaviconURL, "faviconUrl"); err != nil {
		return err
	}
	if err := validateOrganizationLanguages(command.Languages); err != nil {
		return err
	}

	return nil
}

func validatePatchOrganizationCommand(command PatchOrganizationCommand) error {
	if command.DisplayName != nil && strings.TrimSpace(*command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName must not be blank")
	}
	if command.Status != nil && strings.TrimSpace(*command.Status) == "" {
		return apperrors.NewInvalidArgument("status must not be blank")
	}
	if command.WebsiteURL != nil {
		if err := validateOptionalURL(*command.WebsiteURL, "websiteUrl"); err != nil {
			return err
		}
	}
	if command.LogoURL != nil {
		if err := validateOptionalURL(*command.LogoURL, "logoUrl"); err != nil {
			return err
		}
	}
	if command.LogoDarkURL != nil {
		if err := validateOptionalURL(*command.LogoDarkURL, "logoDarkUrl"); err != nil {
			return err
		}
	}
	if command.FaviconURL != nil {
		if err := validateOptionalURL(*command.FaviconURL, "faviconUrl"); err != nil {
			return err
		}
	}
	if command.Languages != nil {
		if err := validateOrganizationLanguages(command.Languages); err != nil {
			return err
		}
	}

	return nil
}

func validateUpdateOrganizationSettingsCommand(command UpdateOrganizationSettingsCommand) error {
	if err := validateOrganizationLanguages(command.Languages); err != nil {
		return err
	}

	seenCountryCodes := map[string]struct{}{}
	for index, countryCode := range command.CountryCodes {
		value := strings.TrimSpace(countryCode)
		if value == "" {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "settings country code must not be empty", map[string]any{metaKeyIndex: index})
		}
		// ISO 3166-1 alpha-2 only; membership is not checked because the codebase
		// has no shared country directory.
		if !phonefmt.IsISOCountryCode(value) {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"settings country code must be an ISO 3166-1 alpha-2 code (uppercase)",
				map[string]any{metaKeyIndex: index, metaKeyField: "countryCodes", metaKeyValue: value, metaKeyFormat: "iso3166-alpha2"},
			)
		}
		if _, ok := seenCountryCodes[value]; ok {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "settings country code must be unique", map[string]any{metaKeyIndex: index, metaKeyField: "countryCodes", "countryCode": value})
		}
		seenCountryCodes[value] = struct{}{}
	}

	seenAccountItems := map[string]struct{}{}
	for index, item := range command.AccountItems {
		if err := validateOrganizationAccountItem(index, item, seenAccountItems); err != nil {
			return err
		}
	}

	return nil
}

// validateOrganizationLanguages enforces the embedded-translation whitelist.
// Languages are a code-level capability (14 embedded translation packages);
// tenant settings may only enable a supported subset. Create uses the same rule
// as the settings endpoint because both persist organization_settings.languages.
func validateOrganizationLanguages(languages []string) error {
	seenLanguages := map[string]struct{}{}
	for index, language := range languages {
		value := strings.TrimSpace(language)
		if value == "" {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "settings language must not be empty", map[string]any{metaKeyIndex: index})
		}
		if !platformi18n.IsSupportedLanguage(value) {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"settings language is not supported",
				map[string]any{metaKeyIndex: index, "language": value, "allowed": platformi18n.SupportedLanguages},
			)
		}
		if _, ok := seenLanguages[value]; ok {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "settings language must be unique", map[string]any{metaKeyIndex: index, "language": value})
		}
		seenLanguages[value] = struct{}{}
	}

	return nil
}

// accountItems 的规则枚举与前端 AccountItemTable 下拉保持同一封闭集合；
// 空串表示沿用默认（Public / Self），与其他可选项一样合法。
var (
	allowedAccountItemViewRules   = []string{"", "Public", "Self", "Admin"}
	allowedAccountItemModifyRules = []string{"", "Self", "Admin", "Immutable"}
)

func validateOrganizationAccountItem(index int, item AccountItemDTO, seen map[string]struct{}) error {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		return apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "settings account item name is required", map[string]any{metaKeyIndex: index})
	}
	if _, ok := seen[name]; ok {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "settings account item name must be unique", map[string]any{metaKeyIndex: index, "name": name})
	}
	seen[name] = struct{}{}

	if item.Tab != "" && strings.TrimSpace(item.Tab) == "" {
		return apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "settings account item tab must not be blank", map[string]any{metaKeyIndex: index})
	}

	if !containsStringValue(allowedAccountItemViewRules, item.ViewRule) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"settings account item viewRule must be one of Public, Self, Admin",
			map[string]any{metaKeyIndex: index, metaKeyField: "viewRule", metaKeyValue: item.ViewRule, "allowed": allowedAccountItemViewRules},
		)
	}
	if !containsStringValue(allowedAccountItemModifyRules, item.ModifyRule) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"settings account item modifyRule must be one of Self, Admin, Immutable",
			map[string]any{metaKeyIndex: index, metaKeyField: "modifyRule", metaKeyValue: item.ModifyRule, "allowed": allowedAccountItemModifyRules},
		)
	}
	if regex := strings.TrimSpace(item.Regex); regex != "" {
		if _, err := regexp.Compile(regex); err != nil {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"settings account item regex is not a valid expression",
				map[string]any{metaKeyIndex: index, metaKeyField: "regex", metaKeyValue: item.Regex, "reason": err.Error()},
			)
		}
	}

	return nil
}

func containsStringValue(values []string, value string) bool {
	for _, allowed := range values {
		if value == allowed {
			return true
		}
	}
	return false
}

func validateUpdateOrganizationThemeCommand(command UpdateOrganizationThemeCommand) error {
	if strings.TrimSpace(command.ThemeName) == "" {
		return apperrors.NewInvalidArgument("themeName is required")
	}

	return nil
}

var allowedPasswordDeriveAlgorithms = []string{
	orgdomain.PasswordDeriveAlgorithmHMACSHA256,
	orgdomain.PasswordDeriveAlgorithmHMACSM3,
}

func validateUpdateOrganizationSecurityPolicyCommand(command UpdateOrganizationSecurityPolicyCommand) error {
	if err := validatePasswordDeriveAlgorithm(command.PasswordDeriveAlgorithm); err != nil {
		return err
	}
	if err := validatePositiveNumberField(command.PasswordPolicy, "minLength", "passwordPolicy.minLength"); err != nil {
		return err
	}
	if err := validateNumberRange(command.PasswordPolicy, "minLength", "passwordPolicy.minLength", 6, 128); err != nil {
		return err
	}
	if err := validateNumberRange(command.PasswordPolicy, "passwordHistoryCount", "passwordPolicy.passwordHistoryCount", 0, 24); err != nil {
		return err
	}
	if err := validatePositiveNumberField(command.SessionPolicy, "idleTimeoutMinutes", "sessionPolicy.idleTimeoutMinutes"); err != nil {
		return err
	}

	return nil
}

// 空值表示沿用默认 hmac_sha256；非空值必须是封闭枚举之一。
func validatePasswordDeriveAlgorithm(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	for _, allowed := range allowedPasswordDeriveAlgorithms {
		if value == allowed {
			return nil
		}
	}
	return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
		"passwordDeriveAlgorithm must be one of hmac_sha256, hmac_sm3",
		map[string]any{metaKeyField: "passwordDeriveAlgorithm", metaKeyValue: value, "allowed": allowedPasswordDeriveAlgorithms})
}

// validateNumberRange 对可选数值字段做上下界校验；缺省或非数字交给
// validatePositiveNumberField / 调用方处理。
func validateNumberRange(policy map[string]any, key, fieldName string, min, max float64) error {
	if policy == nil {
		return nil
	}
	value, ok := policy[key]
	if !ok {
		return nil
	}
	number, ok := numericValue(value)
	if !ok {
		return nil
	}
	if number < min || number > max {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("%s must be between %g and %g", fieldName, min, max),
			map[string]any{metaKeyField: fieldName, metaKeyValue: value, "min": min, "max": max})
	}
	return nil
}

func validatePositiveNumberField(policy map[string]any, key, fieldName string) error {
	if policy == nil {
		return nil
	}

	value, ok := policy[key]
	if !ok {
		return nil
	}

	number, ok := numericValue(value)
	if !ok {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fmt.Sprintf("%s must be a number", fieldName), map[string]any{metaKeyField: fieldName, metaKeyValue: value, "expected": "number"})
	}
	if number <= 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fmt.Sprintf("%s must be greater than zero", fieldName), map[string]any{metaKeyField: fieldName, metaKeyValue: value, "min": 1})
	}

	return nil
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func validateOptionalURL(value, fieldName string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fieldName+" must be a valid absolute URL", map[string]any{metaKeyField: fieldName, metaKeyValue: trimmed, metaKeyFormat: "uri"})
	}

	return nil
}
