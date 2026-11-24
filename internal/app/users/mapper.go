package users

import (
	"encoding/json"
	"strconv"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

func toListItem(user *userdomain.User) UserListItem {
	return UserListItem{
		ID:             user.ID,
		OrganizationID: user.OrganizationID,
		Username:       user.Username,
		DisplayName:    user.Profile.DisplayName,
		AvatarURL:      user.Profile.AvatarURL,
		Email:          user.Profile.Email,
		// 规范 9.5 敏感字段表：手机号列表查询时掩码中间 4 位。详情顶层 phone
		// 随本映射同样掩码；表单编辑读 profile.phone（明文，写路径专用）。
		Phone:               getMaskedPhone(user.Profile.Phone),
		UserType:            user.UserType,
		Status:              user.Status,
		IsAdmin:             user.IsAdmin,
		IsForbidden:         user.IsForbidden,
		IsDeleted:           user.IsDeleted,
		IsVerified:          user.IsVerified,
		IsOnline:            user.IsOnline,
		CreatedIp:           user.CreatedIp,
		Tag:                 user.Tag,
		Affiliation:         user.Profile.Affiliation,
		RealName:            user.Profile.RealName,
		Score:               user.Score,
		Balance:             user.Balance,
		BalanceCredit:       user.BalanceCredit,
		BalanceCurrency:     user.BalanceCurrency,
		Karma:               profilePropertyInt(user.Profile.Properties, "karma"),
		Ranking:             profilePropertyInt(user.Profile.Properties, "ranking"),
		SignupApplicationID: user.SignupApplicationID,
		RegisterType:        user.RegisterType,
		RegisterSource:      user.RegisterSource,
		LastSigninAt:        user.LastSigninAt,
		CreatedAt:           user.CreatedAt,
		Revision:            user.Revision,
	}
}

func toDetail(user *userdomain.User, organization *orgdomain.Organization) UserDetail {
	identities := make([]map[string]any, 0, len(user.Identities))
	for i := range user.Identities {
		identity := &user.Identities[i]
		identities = append(identities, map[string]any{
			"id":              identity.ID,
			"providerId":      identity.ProviderID,
			"identityType":    identity.IdentityType,
			"subject":         identity.Subject,
			"providerAccount": identity.ProviderAccount,
			"linkedAt":        identity.LinkedAt,
		})
	}

	mfaMethods := make([]map[string]any, 0, len(user.MfaMethods))
	for _, method := range user.MfaMethods {
		mfaMethods = append(mfaMethods, map[string]any{
			"id":          method.ID,
			"methodType":  method.MethodType,
			"enabled":     method.Enabled,
			"verified":    method.Verified,
			"displayName": method.DisplayName,
			"lastUsedAt":  method.LastUsedAt,
		})
	}

	roleBindings := make([]map[string]any, 0, len(user.RoleBindings))
	for _, binding := range user.RoleBindings {
		roleBindings = append(roleBindings, map[string]any{"roleId": binding.RoleID})
	}

	groupMemberships := make([]map[string]any, 0, len(user.GroupMemberships))
	for _, membership := range user.GroupMemberships {
		groupMemberships = append(groupMemberships, map[string]any{"groupId": membership.GroupID})
	}

	mfaSetupSummary := make([]map[string]any, 0, 5)
	rememberHours, _ := userMfaRememberHours(organization)
	for _, methodType := range []string{mfaMethodApp, mfaMethodEmail, mfaMethodSMS, mfaMethodRadius, mfaMethodPush} {
		mfaSetupSummary = append(mfaSetupSummary, map[string]any{
			"enabled":            isUserMfaSetupEnabled(user, methodType),
			"isPreferred":        normalizeSetupSummaryMethodType(user.PreferredMfaType) == methodType,
			"mfaType":            methodType,
			"secret":             userMfaSetupSecret(user, methodType),
			"countryCode":        userMfaSetupCountryCode(user, methodType),
			"url":                userMfaSetupURL(user, methodType),
			"mfaRememberInHours": rememberHours,
		})
	}

	return UserDetail{
		UserListItem:        toListItem(user),
		ExternalID:          user.ExternalID,
		SignupApplicationID: user.SignupApplicationID,
		RegisterType:        user.RegisterType,
		RegisterSource:      user.RegisterSource,
		PreferredMfaType:    user.PreferredMfaType,
		// Recovery codes are one-time credentials displayed only at setup time
		// (see InitiateMfaSetupResponse). We intentionally return a same-length
		// placeholder slice here so that UI code using `.length` keeps working
		// while no plaintext recovery code is ever exposed via GET.
		RecoveryCodes:   make([]string, len(user.RecoveryCodeHashes)),
		MfaSetupSummary: mfaSetupSummary,
		Profile: map[string]any{
			"displayName":       user.Profile.DisplayName,
			"firstName":         user.Profile.FirstName,
			"lastName":          user.Profile.LastName,
			"avatarUrl":         user.Profile.AvatarURL,
			fieldEmail:          user.Profile.Email,
			"emailVerified":     user.Profile.EmailVerified,
			"phone":             user.Profile.Phone,
			"phoneVerified":     user.Profile.PhoneVerified,
			"countryCode":       user.Profile.CountryCode,
			"region":            user.Profile.Region,
			"location":          user.Profile.Location,
			"birthday":          user.Profile.Birthday,
			"gender":            user.Profile.Gender,
			"bio":               user.Profile.Bio,
			profileKeyHomepage:  profilePropertyString(user.Profile.Properties, profileKeyHomepage),
			profileKeyEducation: profilePropertyString(user.Profile.Properties, profileKeyEducation),
			profileKeyLanguage:  profilePropertyString(user.Profile.Properties, profileKeyLanguage),
			"realName":          user.Profile.RealName,
			"idCardType":        user.Profile.IDCardType,
			// 规范 9.5：证件号永不返回原文。响应只带 Set/Masked 对（凭据族同款
			// 语义）；写路径的空值保留语义见 UpdateUserProfile。
			"idCardSet":             user.Profile.IDCard != "",
			"idCardMasked":          maskCredential(user.Profile.IDCard),
			"passportNoSet":         user.Profile.PassportNo != "",
			"passportNoMasked":      maskCredential(user.Profile.PassportNo),
			"driverLicenseNoSet":    user.Profile.DriverLicenseNo != "",
			"driverLicenseNoMasked": maskCredential(user.Profile.DriverLicenseNo),
			"idCardExpiry":          user.Profile.IDCardExpiry,
			"issueCountry":          user.Profile.IssueCountry,
			"issuingAuthority":      user.Profile.IssuingAuthority,
			"passportPhotoUrl":      user.Profile.PassportPhotoURL,
			"driverLicenseFrontUrl": user.Profile.DriverLicenseFrontURL,
			"driverLicenseBackUrl":  user.Profile.DriverLicenseBackURL,
			profileKeyAffiliation:   user.Profile.Affiliation,
			profileKeyTitle:         profilePropertyString(user.Profile.Properties, profileKeyTitle),
			"idCardFront":           profilePropertyString(user.Profile.Properties, "idCardFront"),
			"idCardBack":            profilePropertyString(user.Profile.Properties, "idCardBack"),
			"idCardWithPerson":      profilePropertyString(user.Profile.Properties, "idCardWithPerson"),
			"properties":            responseProperties(user.Profile.Properties),
		},
		CredentialSummary: map[string]any{
			"hasPassword":           user.Credentials.HasPassword,
			"passwordAlgo":          user.Credentials.PasswordAlgo,
			"passwordUpdatedAt":     user.Credentials.PasswordUpdatedAt,
			"hasAccessKey":          user.Credentials.AccessKey != "",
			"accessKeySet":          user.Credentials.AccessKey != "",
			"accessKeyMasked":       maskCredential(user.Credentials.AccessKey),
			"accessSecretRotatedAt": user.Credentials.AccessSecretRotatedAt,
			"needUpdatePassword":    user.Credentials.NeedUpdatePassword,
		},
		Identities:              identities,
		MfaMethods:              mfaMethods,
		WebauthnCredentialCount: 0, // GetUser 从 WebAuthn 仓储回填真实计数
		RoleBindings:            roleBindings,
		GroupMemberships:        groupMemberships,
		// 同意记录存于用户聚合（ListUserConsents 同源）；撤销即从
		// ApplicationScopes 移除条目，因此现存条目全部为活跃。
		ConsentSummary: map[string]any{
			"totalConsents":  len(user.ApplicationScopes),
			"activeConsents": len(user.ApplicationScopes),
		},
		BalanceCredit:     user.BalanceCredit,
		BalanceCurrency:   user.BalanceCurrency,
		IpWhitelist:       user.IpWhitelist,
		Addresses:         user.Addresses,
		ManagedAccounts:   toManagedAccountItems(user.ManagedAccounts),
		FaceIds:           user.FaceIds,
		Cart:              user.Cart,
		MfaItems:          toMfaItemEntries(user.MfaItems),
		MfaAccounts:       toMfaAccountItems(user.MfaAccounts),
		ApplicationScopes: user.ApplicationScopes,
		Custom1:           user.Custom1,
		Custom2:           user.Custom2,
		Custom3:           user.Custom3,
		Custom4:           user.Custom4,
		Custom5:           user.Custom5,
		Custom6:           user.Custom6,
		Custom7:           user.Custom7,
		Custom8:           user.Custom8,
		Custom9:           user.Custom9,
		Custom10:          user.Custom10,
		Revision:          user.Revision,
	}
}

func toManagedAccountItems(accounts []userdomain.ManagedAccount) []ManagedAccountItem {
	items := make([]ManagedAccountItem, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, ManagedAccountItem{
			Application:    account.Application,
			Username:       account.Username,
			PasswordSet:    account.Password != "",
			PasswordMasked: maskCredential(account.Password),
			SigninURL:      account.SigninUrl,
		})
	}
	return items
}

func toMfaItemEntries(entries []userdomain.MfaItem) []MfaItemEntry {
	items := make([]MfaItemEntry, 0, len(entries))
	for _, entry := range entries {
		items = append(items, MfaItemEntry{
			MfaType:      entry.MfaType,
			SecretSet:    entry.Secret != "",
			SecretMasked: maskCredential(entry.Secret),
		})
	}
	return items
}

func toMfaAccountItems(accounts []userdomain.MfaAccount) []MfaAccountItem {
	items := make([]MfaAccountItem, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, MfaAccountItem{
			AccountName:     account.AccountName,
			Issuer:          account.Issuer,
			SecretKeySet:    account.SecretKey != "",
			SecretKeyMasked: maskCredential(account.SecretKey),
			Origin:          account.Origin,
		})
	}
	return items
}

func maskCredential(value string) string {
	if value == "" {
		return ""
	}
	return "******"
}

// responseProperties 过滤 properties 里的证件号原文键。这三个键如今是自定义
// 属性保留键（custom_attributes.go），新写入会被拒绝，但存量 JSONB 可能仍
// 携带历史明文；规范 9.5 要求证件号永不返回原文，因此响应侧统一剔除。
// 证件号的权威存储是 user_profiles 的独立列（响应以 Set/Masked 对呈现）。
func responseProperties(properties map[string]any) map[string]any {
	if properties == nil {
		return nil
	}
	filtered := make(map[string]any, len(properties))
	for key, value := range properties {
		switch key {
		case "idCard", "passportNo", "driverLicenseNo":
			continue
		}
		filtered[key] = value
	}
	return filtered
}

func profilePropertyString(properties map[string]any, key string) string {
	value, _ := properties[key].(string)
	return value
}

func profilePropertyInt(properties map[string]any, key string) int {
	switch value := properties[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(value)
		return parsed
	default:
		return 0
	}
}

func normalizeSetupSummaryMethodType(methodType string) string {
	if methodType == mfaMethodTOTP {
		return mfaMethodApp
	}
	return methodType
}

func isUserMfaSetupEnabled(user *userdomain.User, methodType string) bool {
	normalized := normalizeSetupSummaryMethodType(methodType)
	for _, method := range user.MfaMethods {
		if normalizeSetupSummaryMethodType(method.MethodType) == normalized && method.Enabled {
			return true
		}
	}

	if normalized == mfaMethodApp {
		return user.TotpSecret != ""
	}

	return false
}

func userMfaSetupSecret(user *userdomain.User, methodType string) string {
	switch normalizeSetupSummaryMethodType(methodType) {
	case mfaMethodEmail:
		if user.Profile.Email == "" {
			return ""
		}
		return getMaskedEmail(user.Profile.Email)
	case mfaMethodSMS:
		if user.Profile.Phone == "" {
			return ""
		}
		return getMaskedPhone(user.Profile.Phone)
	case mfaMethodRadius:
		if user.MfaRadiusUsername == "" {
			return ""
		}
		return getMaskedEmail(user.MfaRadiusUsername)
	case mfaMethodPush:
		if user.MfaPushReceiver == "" {
			return ""
		}
		return getMaskedEmail(user.MfaPushReceiver)
	default:
		return ""
	}
}

func userMfaSetupCountryCode(user *userdomain.User, methodType string) string {
	if normalizeSetupSummaryMethodType(methodType) == mfaMethodSMS {
		return user.Profile.CountryCode
	}
	return ""
}

func userMfaSetupURL(user *userdomain.User, methodType string) string {
	switch normalizeSetupSummaryMethodType(methodType) {
	case mfaMethodRadius:
		return user.MfaRadiusProvider
	case mfaMethodPush:
		return user.MfaPushProvider
	default:
		return ""
	}
}
