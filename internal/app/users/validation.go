package users

import (
	customattributes "github.com/guliangiam/guliangiam/internal/domain/customattributes"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/phonefmt"
)

func validateCreateUserCommand(command CreateUserWithProfileCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Username) == "" {
		return apperrors.NewInvalidArgument("username is required")
	}
	if strings.TrimSpace(command.UserType) == "" {
		return apperrors.NewInvalidArgument("userType is required")
	}
	if strings.TrimSpace(command.Status) == "" {
		return apperrors.NewInvalidArgument("status is required")
	}
	if err := validateUserStatus(command.Status); err != nil {
		return err
	}
	if strings.TrimSpace(command.Profile.DisplayName) == "" {
		return apperrors.NewInvalidArgument("profile.displayName is required")
	}
	if err := validateOptionalEmail(command.Profile.Email, "profile.email"); err != nil {
		return err
	}
	if err := validateOptionalURL(command.Profile.AvatarURL, "profile.avatarUrl"); err != nil {
		return err
	}
	if err := validateOptionalURL(command.Profile.Homepage, "profile.homepage"); err != nil {
		return err
	}
	if err := phonefmt.ValidateCountryCode(command.Profile.CountryCode, "profile.countryCode"); err != nil {
		return err
	}
	if err := phonefmt.ValidatePhone(command.Profile.Phone, "profile.phone"); err != nil {
		return err
	}

	roleCommands := make([]ReplaceUserRoleBindingCommand, 0, len(command.RoleBindingIDs))
	for _, roleID := range command.RoleBindingIDs {
		roleCommands = append(roleCommands, ReplaceUserRoleBindingCommand{RoleID: roleID})
	}
	if err := validateReplaceUserRoleBindingCommands(roleCommands); err != nil {
		return err
	}

	groupCommands := make([]ReplaceUserGroupMembershipCommand, 0, len(command.GroupIDs))
	for _, groupID := range command.GroupIDs {
		groupCommands = append(groupCommands, ReplaceUserGroupMembershipCommand{GroupID: groupID})
	}
	if err := validateReplaceUserGroupMembershipCommands(groupCommands); err != nil {
		return err
	}

	return nil
}

func validatePatchUserCommand(command PatchUserCommand) error {
	// 规范 4.14.1：语法可解析但语义非法 → 422（空白串是可绑定值，不是畸形
	// 报文）；details.field 按 4.15.3 给出字段名。
	if command.UserType != nil && strings.TrimSpace(*command.UserType) == "" {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "userType must not be blank", map[string]any{metaKeyField: "userType", metaKeyValue: *command.UserType, "reason": "blank"})
	}
	if command.Status != nil && strings.TrimSpace(*command.Status) == "" {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "status must not be blank", map[string]any{metaKeyField: "status", metaKeyValue: *command.Status, "reason": "blank"})
	}
	if command.Status != nil {
		if err := validateUserStatus(*command.Status); err != nil {
			return err
		}
	}

	return nil
}

// validateUserStatus 以 usersdomain.IsValid 为唯一枚举白名单（事实源
// contracts/registries/state-machines.yaml §users_status，DB 侧由 ck_users_status
// 承载）。非法值必须在 app 层拒绝为 422；否则会直达 DB CHECK 违约变成 500。
// 校验原始值而非 TrimSpace 结果：CHECK 约束比较的就是入库原值。
func validateUserStatus(status string) error {
	if userdomain.IsValid(status) {
		return nil
	}
	allowed := make([]string, 0, len(userdomain.AllUsersState))
	for _, state := range userdomain.AllUsersState {
		allowed = append(allowed, string(state))
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"status must be a valid user state",
		map[string]any{metaKeyField: "status", metaKeyValue: status, "allowed": allowed},
	)
}

func validateUpdateUserProfileCommand(command UpdateUserProfileCommand) error {
	if err := validateOptionalEmail(command.Email, fieldEmail); err != nil {
		return err
	}
	if err := validateOptionalURL(command.AvatarURL, "avatarUrl"); err != nil {
		return err
	}
	if err := phonefmt.ValidateCountryCode(command.CountryCode, "profile.countryCode"); err != nil {
		return err
	}
	if err := phonefmt.ValidatePhone(command.Phone, "profile.phone"); err != nil {
		return err
	}
	if err := validateOptionalDate(command.IDCardExpiry, "idCardExpiry"); err != nil {
		return err
	}
	if err := validateOptionalMaxLength(command.IssueCountry, "issueCountry", maxIssueCountryLength); err != nil {
		return err
	}
	if err := validateOptionalMaxLength(command.PassportNo, "passportNo", maxDocumentNumberLength); err != nil {
		return err
	}
	if err := validateOptionalMaxLength(command.DriverLicenseNo, "driverLicenseNo", maxDocumentNumberLength); err != nil {
		return err
	}
	if err := validateOptionalMaxLength(command.IssuingAuthority, "issuingAuthority", maxIssuingAuthorityLength); err != nil {
		return err
	}
	if err := validateProfileProperties(command.Properties); err != nil {
		return err
	}
	if value, exists := command.Properties[profileKeyHomepage]; exists {
		homepage, ok := value.(string)
		if !ok {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "profile.homepage must be a string", map[string]any{metaKeyField: "profile.homepage", "expected": "string"})
		}
		if err := validateOptionalURL(homepage, "profile.homepage"); err != nil {
			return err
		}
	}

	return nil
}

// 自定义属性没有独立的定义表，后端只能校验结构约束；类型/必填/选项等
// 语义校验依赖未来的属性定义契约（见报告中的结构性缺口）。
const (
	maxProfileProperties        = 100
	maxProfilePropertyKeyLength = 64
	profilePropertiesFieldName  = "profile.properties"
)

// 证件字段上限与 contracts/schema 的 VARCHAR 宽度一致（20_identity.yaml），
// id_card_expiry 为 DATE 列，只接受 YYYY-MM-DD。
const (
	maxIssueCountryLength     = 8
	maxDocumentNumberLength   = 64
	maxIssuingAuthorityLength = 255
	canonicalDateFormat       = "2006-01-02"
)

func validateProfileProperties(properties map[string]any) error {
	if len(properties) > maxProfileProperties {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"profile.properties accepts at most 100 entries",
			map[string]any{metaKeyField: profilePropertiesFieldName, "max": maxProfileProperties},
		)
	}
	for key := range properties {
		if strings.TrimSpace(key) == "" {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"profile.properties key must not be blank",
				map[string]any{metaKeyField: profilePropertiesFieldName, "reason": "blank_key"},
			)
		}
		if len(key) > maxProfilePropertyKeyLength {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"profile.properties key must be at most 64 characters",
				map[string]any{metaKeyField: profilePropertiesFieldName, "max": maxProfilePropertyKeyLength},
			)
		}
	}
	return nil
}

func validateUpdateUserCredentialsCommand(command UpdateUserCredentialsCommand) error {
	password := strings.TrimSpace(command.Password)
	if password == "" && !command.RotateAccessSecret && command.NeedUpdatePassword == nil {
		return apperrors.NewInvalidArgument("credentials update requires password, rotateAccessSecret or needUpdatePassword")
	}

	return nil
}

func validateOptionalEmail(value, fieldName string) error {
	email := strings.TrimSpace(value)
	if email == "" {
		return nil
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fieldName+" must be a valid email address", map[string]any{metaKeyField: fieldName, metaKeyValue: email, metaKeyFormat: "email"})
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
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fieldName+" must be a valid absolute URL", map[string]any{metaKeyField: fieldName, metaKeyValue: trimmed, metaKeyFormat: "uri"})
	}

	return nil
}

// validateOptionalDate enforces the canonical YYYY-MM-DD form required by the
// DATE columns; empty values are treated as "not set".
func validateOptionalDate(value, fieldName string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parsed, err := time.Parse(canonicalDateFormat, trimmed)
	if err != nil || parsed.Format(canonicalDateFormat) != trimmed {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fieldName+" must be a YYYY-MM-DD date", map[string]any{metaKeyField: fieldName, metaKeyValue: trimmed, metaKeyFormat: "date"})
	}

	return nil
}

// validateOptionalMaxLength bounds a text field to the column width; the
// comparison uses runes so multi-byte authority names are counted the same
// way PostgreSQL counts VARCHAR characters.
func validateOptionalMaxLength(value, fieldName string, maxLength int) error {
	trimmed := strings.TrimSpace(value)
	if utf8.RuneCountInString(trimmed) <= maxLength {
		return nil
	}
	return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fieldName+" must be at most "+strconv.Itoa(maxLength)+" characters", map[string]any{metaKeyField: fieldName, "max": maxLength})
}

func validateReplaceUserIdentitiesCommands(commands []ReplaceUserIdentityCommand) error {
	seenIDs := map[string]struct{}{}
	seenKeys := map[string]struct{}{}

	for index, command := range commands {
		providerID := strings.TrimSpace(command.ProviderID)
		identityType := strings.TrimSpace(command.IdentityType)
		subject := strings.TrimSpace(command.Subject)

		if providerID == "" {
			return apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "identity providerId is required", map[string]any{metaKeyIndex: index})
		}
		if identityType == "" {
			return apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "identity identityType is required", map[string]any{metaKeyIndex: index})
		}
		if subject == "" {
			return apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "identity subject is required", map[string]any{metaKeyIndex: index})
		}

		if command.ID != "" {
			if _, ok := seenIDs[command.ID]; ok {
				return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "identity id must be unique", map[string]any{metaKeyIndex: index, "id": command.ID})
			}
			seenIDs[command.ID] = struct{}{}
		}

		key := providerID + "|" + identityType + "|" + subject
		if _, ok := seenKeys[key]; ok {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "identity providerId, identityType and subject combination must be unique", map[string]any{metaKeyIndex: index})
		}
		seenKeys[key] = struct{}{}
	}

	return nil
}

func validateReplaceUserMfaMethodsCommands(commands []ReplaceUserMfaMethodCommand) error {
	seenIDs := map[string]struct{}{}
	seenMethodTypes := map[string]struct{}{}

	for index, command := range commands {
		methodType := strings.TrimSpace(command.MethodType)
		if methodType == "" {
			return apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "mfa methodType is required", map[string]any{metaKeyIndex: index})
		}
		if !isAllowedUserMfaMethodType(methodType) {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"mfa methodType is not supported",
				map[string]any{
					metaKeyIndex: index,
					"methodType": methodType,
					"allowed":    append([]string{}, allowedMfaSetupTypes...),
				},
			)
		}

		if command.ID != "" {
			if _, ok := seenIDs[command.ID]; ok {
				return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "mfa method id must be unique", map[string]any{metaKeyIndex: index, "id": command.ID})
			}
			seenIDs[command.ID] = struct{}{}
		}

		if _, ok := seenMethodTypes[normalizeExistingUserMfaMethodType(methodType)]; ok {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "mfa methodType must be unique", map[string]any{metaKeyIndex: index, "methodType": methodType})
		}
		seenMethodTypes[normalizeExistingUserMfaMethodType(methodType)] = struct{}{}
	}

	return nil
}

// isAllowedUserMfaMethodType keeps the replace path inside the same closed set
// the setup flow dispatches on, so the documented enum cannot drift from what
// the server actually accepts.
func isAllowedUserMfaMethodType(methodType string) bool {
	for _, allowed := range allowedMfaSetupTypes {
		if methodType == allowed {
			return true
		}
	}
	return false
}

func validateUpdateUserPreferredMfaCommand(command UpdateUserPreferredMfaCommand) error {
	if strings.TrimSpace(command.UserID) == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if strings.TrimSpace(command.MfaType) == "" {
		return apperrors.NewInvalidArgument("mfaType is required")
	}

	return nil
}

func validateInitiateUserMfaSetupCommand(command InitiateUserMfaSetupCommand) error {
	if strings.TrimSpace(command.UserID) == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if strings.TrimSpace(command.MfaType) == "" {
		return apperrors.NewInvalidArgument("mfaType is required")
	}

	return nil
}

func validateVerifyUserMfaSetupCommand(command VerifyUserMfaSetupCommand) error {
	if strings.TrimSpace(command.UserID) == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if strings.TrimSpace(command.MfaType) == "" {
		return apperrors.NewInvalidArgument("mfaType is required")
	}
	if strings.TrimSpace(command.Passcode) == "" {
		return apperrors.NewInvalidArgument("passcode is required")
	}

	return nil
}

func validateEnableUserMfaSetupCommand(command EnableUserMfaSetupCommand) error {
	if strings.TrimSpace(command.UserID) == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if strings.TrimSpace(command.MfaType) == "" {
		return apperrors.NewInvalidArgument("mfaType is required")
	}

	hasRecoveryCode := false
	for _, code := range command.RecoveryCodes {
		if strings.TrimSpace(code) != "" {
			hasRecoveryCode = true
			break
		}
	}
	if !hasRecoveryCode {
		return apperrors.NewInvalidArgument("recoveryCodes is required")
	}

	return nil
}

func validateReplaceUserRoleBindingCommands(commands []ReplaceUserRoleBindingCommand) error {
	seenRoleIDs := map[string]struct{}{}

	for index, command := range commands {
		roleID := strings.TrimSpace(command.RoleID)
		if roleID == "" {
			return apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "role binding roleId is required", map[string]any{metaKeyIndex: index})
		}

		if _, ok := seenRoleIDs[roleID]; ok {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "role binding roleId must be unique", map[string]any{metaKeyIndex: index, "roleId": roleID})
		}
		seenRoleIDs[roleID] = struct{}{}
	}

	return nil
}

func validateReplaceUserGroupMembershipCommands(commands []ReplaceUserGroupMembershipCommand) error {
	seenGroupIDs := map[string]struct{}{}

	for index, command := range commands {
		groupID := strings.TrimSpace(command.GroupID)
		if groupID == "" {
			return apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "group membership groupId is required", map[string]any{metaKeyIndex: index})
		}

		if _, ok := seenGroupIDs[groupID]; ok {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "group membership groupId must be unique", map[string]any{metaKeyIndex: index, "groupId": groupID})
		}
		seenGroupIDs[groupID] = struct{}{}
	}

	return nil
}

// ValidateProfilePropertiesAgainstDefinitions applies the tenant's custom
// attribute definitions to a submitted profile: unregistered keys, disabled or
// read-only attributes, wrong types, enum mismatches and missing required
// values are rejected with 422 (OPEN-API-59).
func ValidateProfilePropertiesAgainstDefinitions(properties map[string]any, definitions []customattributes.Definition) error {
	if len(definitions) == 0 {
		return nil
	}
	return customattributes.ValidateProperties(properties, definitions)
}
