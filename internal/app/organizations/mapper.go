package organizations

import orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"

func toListItem(organization *orgdomain.Organization) OrganizationListItem {
	return OrganizationListItem{
		ID:                   organization.ID,
		ParentID:             organization.ParentID,
		RegionCode:           organization.RegionCode,
		Slug:                 organization.Slug,
		DisplayName:          organization.DisplayName,
		WebsiteURL:           organization.WebsiteURL,
		LogoURL:              organization.LogoURL,
		DefaultApplicationID: organization.Settings.DefaultApplicationID,
		Status:               organization.Status,
		PasswordType:         organization.PasswordType,
		Balance:              organization.Balance,
		Currency:             organization.Currency,
		CreatedAt:            organization.CreatedAt,
		UpdatedAt:            organization.UpdatedAt,
	}
}

func toDetail(organization *orgdomain.Organization) OrganizationDetail {
	return OrganizationDetail{
		OrganizationListItem:   toListItem(organization),
		LogoDarkURL:            organization.LogoDarkURL,
		FaviconURL:             organization.FaviconURL,
		InitScore:              organization.InitScore,
		DefaultAvatar:          organization.DefaultAvatar,
		PasswordObfuscatorType: organization.PasswordObfuscatorType,
		DisableSignin:          organization.DisableSignin,
		PasswordOptions:        organization.PasswordOptions,
		EnableSoftDeletion:     organization.EnableSoftDeletion,
		IpWhitelist:            organization.IpWhitelist,
		InitiateLoginURL:       organization.InitiateLoginURL,
		Settings: map[string]any{
			"languages":            organization.Settings.Languages,
			"defaultApplicationId": organization.Settings.DefaultApplicationID,
			"accountItems":         organization.Settings.AccountItems,
			"navPreferences":       organization.Settings.NavPreferences,
			"featureFlags":         organization.Settings.FeatureFlags,
			"countryCodes":         organization.Settings.CountryCodes,
			"userTypes":            organization.Settings.UserTypes,
			"tags":                 organization.Settings.Tags,
			"mfaItems":             organization.Settings.MfaItems,
			"accountMenu":          organization.Settings.AccountMenu,
			"mfaRememberInHours":   organization.Settings.MfaRememberInHours,
			"dcrPolicy":            organization.Settings.DcrPolicy,
		},
		Theme: map[string]any{
			"themeName": organization.Theme.ThemeName,
			"tokens":    organization.Theme.Tokens,
			"customCss": organization.Theme.CustomCSS,
			"assets":    organization.Theme.Assets,
		},
		SecurityPolicySummary: map[string]any{
			"passwordPolicy":          organization.Security.PasswordPolicy,
			"passwordDeriveAlgorithm": organization.Security.EffectivePasswordDeriveAlgorithm(),
			"mfaPolicy":               organization.Security.MfaPolicy,
			"ipPolicy":                organization.Security.IPPolicy,
			"sessionPolicy":           organization.Security.SessionPolicy,
			"riskPolicy":              organization.Security.RiskPolicy,
			"hasMasterKey":            organization.Security.HasMasterKey,
		},
		Revision: organization.Revision,
		// Phase 2: Extended fields
		PasswordType:                 organization.PasswordType,
		PasswordSaltSet:              organization.PasswordSalt != "",
		PasswordSaltMasked:           maskCredential(organization.PasswordSalt),
		PasswordObfuscatorKeySet:     organization.PasswordObfuscatorKey != "",
		PasswordObfuscatorKeyMasked:  maskCredential(organization.PasswordObfuscatorKey),
		EnableDeposit:                organization.EnableDeposit,
		BalanceCredit:                organization.BalanceCredit,
		Mu:                           organization.Mu,
		DefaultQuota:                 organization.DefaultQuota,
		DefaultCurrency:              organization.DefaultCurrency,
		EnableDarkLogo:               organization.EnableDarkLogo,
		HasPrivilegeConsent:          organization.HasPrivilegeConsent,
		IsProfilePublic:              organization.IsProfilePublic,
		UseEmailAsUsername:           organization.UseEmailAsUsername,
		UsePermanentAvatar:           organization.UsePermanentAvatar,
		EnableTour:                   organization.EnableTour,
		MasterPasswordSet:            organization.MasterPassword != "",
		MasterPasswordMasked:         maskCredential(organization.MasterPassword),
		MasterVerificationCodeSet:    organization.MasterVerificationCode != "",
		MasterVerificationCodeMasked: maskCredential(organization.MasterVerificationCode),
		DefaultPasswordSet:           organization.DefaultPassword != "",
		DefaultPasswordMasked:        maskCredential(organization.DefaultPassword),
		LdapAttributes:               organization.LdapAttributes,
		KerberosRealm:                organization.KerberosRealm,
		KerberosKdcHost:              organization.KerberosKdcHost,
		KerberosKeytabSet:            organization.KerberosKeytab != "",
		KerberosKeytabMasked:         maskCredential(organization.KerberosKeytab),
		KerberosServiceName:          organization.KerberosServiceName,
	}
}

// maskCredential mirrors the certs/providers read-side convention: short
// secrets are fully masked, longer ones keep the first and last two characters
// so an operator can recognise the value without reading it. Empty stays empty.
//
// Deliberately duplicated from internal/app/certs/mapper.go and
// internal/app/providers/mapper.go: extracting a shared helper crosses package
// boundaries (platform/** is out of scope for this hotfix) and is registered as
// a follow-up. Runes are used so a multi-byte secret cannot be sliced into
// invalid UTF-8.
func maskCredential(secret string) string {
	if secret == "" {
		return ""
	}
	runes := []rune(secret)
	if len(runes) < 6 {
		return "******"
	}
	return string(runes[:2]) + "***" + string(runes[len(runes)-2:])
}
