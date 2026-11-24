package organizations

import (
	apporganizations "github.com/guliangiam/guliangiam/internal/app/organizations"
)

type ListOrganizationsRequest struct {
	Page     int
	PageSize int
	Keyword  string
	Status   string
}

type GetOrganizationRequest struct {
	ID string
}

type GetOrganizationBySlugRequest struct {
	Slug string
}

type CreateOrganizationRequest struct {
	Slug                 string   `json:"slug"`
	ParentID             string   `json:"parentId,omitempty"`
	RegionCode           string   `json:"regionCode,omitempty"`
	DisplayName          string   `json:"displayName"`
	WebsiteURL           string   `json:"websiteUrl,omitempty"`
	LogoURL              string   `json:"logoUrl,omitempty"`
	LogoDarkURL          string   `json:"logoDarkUrl,omitempty"`
	FaviconURL           string   `json:"faviconUrl,omitempty"`
	Status               string   `json:"status"`
	Languages            []string `json:"languages,omitempty"`
	InitScore            int      `json:"initScore,omitempty"`
	DefaultApplicationID string   `json:"defaultApplicationId,omitempty"`

	// Optional: branding
	DefaultAvatar  string `json:"defaultAvatar,omitempty"`
	EnableDarkLogo bool   `json:"enableDarkLogo,omitempty"`

	// Optional: password policy
	PasswordType           string   `json:"passwordType,omitempty"`
	PasswordSalt           string   `json:"passwordSalt,omitempty"`
	PasswordObfuscatorType string   `json:"passwordObfuscatorType,omitempty"`
	PasswordObfuscatorKey  string   `json:"passwordObfuscatorKey,omitempty"`
	PasswordOptions        []string `json:"passwordOptions,omitempty"`
	DefaultPassword        string   `json:"defaultPassword,omitempty"`
	MasterPassword         string   `json:"masterPassword,omitempty"`
	MasterVerificationCode string   `json:"masterVerificationCode,omitempty"`

	// Optional: security & signin
	DisableSignin       bool `json:"disableSignin,omitempty"`
	EnableTour          bool `json:"enableTour,omitempty"`
	HasPrivilegeConsent bool `json:"hasPrivilegeConsent,omitempty"`
	IsProfilePublic     bool `json:"isProfilePublic,omitempty"`
	UseEmailAsUsername  bool `json:"useEmailAsUsername,omitempty"`
}

type PatchOrganizationRequest struct {
	DisplayName            *string  `json:"displayName,omitempty"`
	WebsiteURL             *string  `json:"websiteUrl,omitempty"`
	LogoURL                *string  `json:"logoUrl,omitempty"`
	LogoDarkURL            *string  `json:"logoDarkUrl,omitempty"`
	FaviconURL             *string  `json:"faviconUrl,omitempty"`
	Status                 *string  `json:"status,omitempty"`
	ParentID               *string  `json:"parentId,omitempty"`
	RegionCode             *string  `json:"regionCode,omitempty"`
	Languages              []string `json:"languages,omitempty"`
	InitScore              *int     `json:"initScore,omitempty"`
	DefaultAvatar          *string  `json:"defaultAvatar,omitempty"`
	PasswordObfuscatorType *string  `json:"passwordObfuscatorType,omitempty"`
	DisableSignin          *bool    `json:"disableSignin,omitempty"`
	PasswordOptions        []string `json:"passwordOptions,omitempty"`
	EnableSoftDeletion     *bool    `json:"enableSoftDeletion,omitempty"`
	IpWhitelist            *string  `json:"ipWhitelist,omitempty"`
	InitiateLoginURL       *string  `json:"initiateLoginUrl,omitempty"`
	Balance                *float64 `json:"balance,omitempty"`
	Currency               *string  `json:"currency,omitempty"`
	Revision               int64    `json:"revision,omitempty"`
	// Phase 2: Extended fields
	PasswordType           *string  `json:"passwordType,omitempty"`
	PasswordSalt           *string  `json:"passwordSalt,omitempty"`
	PasswordObfuscatorKey  *string  `json:"passwordObfuscatorKey,omitempty"`
	EnableDeposit          *bool    `json:"enableDeposit,omitempty"`
	BalanceCredit          *float64 `json:"balanceCredit,omitempty"`
	Mu                     *int     `json:"mu,omitempty"`
	DefaultQuota           *int     `json:"defaultQuota,omitempty"`
	DefaultCurrency        *string  `json:"defaultCurrency,omitempty"`
	EnableDarkLogo         *bool    `json:"enableDarkLogo,omitempty"`
	HasPrivilegeConsent    *bool    `json:"hasPrivilegeConsent,omitempty"`
	IsProfilePublic        *bool    `json:"isProfilePublic,omitempty"`
	UseEmailAsUsername     *bool    `json:"useEmailAsUsername,omitempty"`
	UsePermanentAvatar     *bool    `json:"usePermanentAvatar,omitempty"`
	EnableTour             *bool    `json:"enableTour,omitempty"`
	MasterPassword         *string  `json:"masterPassword,omitempty"`
	MasterVerificationCode *string  `json:"masterVerificationCode,omitempty"`
	DefaultPassword        *string  `json:"defaultPassword,omitempty"`
	LdapAttributes         []string `json:"ldapAttributes,omitempty"`
	KerberosRealm          *string  `json:"kerberosRealm,omitempty"`
	KerberosKdcHost        *string  `json:"kerberosKdcHost,omitempty"`
	KerberosKeytab         *string  `json:"kerberosKeytab,omitempty"`
	KerberosServiceName    *string  `json:"kerberosServiceName,omitempty"`
}

type UpdateOrganizationSettingsRequest struct {
	Languages            []string                         `json:"languages,omitempty"`
	DefaultApplicationID string                           `json:"defaultApplicationId,omitempty"`
	AccountItems         []OrganizationAccountItemRequest `json:"accountItems,omitempty"`
	NavPreferences       map[string]any                   `json:"navPreferences,omitempty"`
	FeatureFlags         map[string]any                   `json:"featureFlags,omitempty"`
	CountryCodes         []string                         `json:"countryCodes,omitempty"`
	UserTypes            []string                         `json:"userTypes,omitempty"`
	Tags                 []string                         `json:"tags,omitempty"`
	// Phase 2: Extended settings fields
	MfaItems           []OrganizationMfaItemRequest `json:"mfaItems,omitempty"`
	AccountMenu        *string                      `json:"accountMenu,omitempty"`
	MfaRememberInHours *int                         `json:"mfaRememberInHours,omitempty"`
	DcrPolicy          *string                      `json:"dcrPolicy,omitempty"`
}

type UpdateOrganizationThemeRequest struct {
	ThemeName string         `json:"themeName,omitempty"`
	Tokens    map[string]any `json:"tokens,omitempty"`
	CustomCSS string         `json:"customCss,omitempty"`
	Assets    map[string]any `json:"assets,omitempty"`
}

type UpdateOrganizationSecurityPolicyRequest struct {
	PasswordPolicy          map[string]any `json:"passwordPolicy,omitempty"`
	PasswordDeriveAlgorithm string         `json:"passwordDeriveAlgorithm,omitempty"`
	MfaPolicy               map[string]any `json:"mfaPolicy,omitempty"`
	IPPolicy                map[string]any `json:"ipPolicy,omitempty"`
	SessionPolicy           map[string]any `json:"sessionPolicy,omitempty"`
	RiskPolicy              map[string]any `json:"riskPolicy,omitempty"`
}

type OrganizationAccountItemRequest struct {
	Name       string `json:"name"`
	Visible    bool   `json:"visible"`
	ViewRule   string `json:"viewRule,omitempty"`
	ModifyRule string `json:"modifyRule,omitempty"`
	Regex      string `json:"regex,omitempty"`
	Tab        string `json:"tab,omitempty"`
}

type OrganizationMfaItemRequest struct {
	Name string `json:"name"`
	Rule string `json:"rule,omitempty"`
}

func (r CreateOrganizationRequest) toCommand() createOrganizationCommand {
	return createOrganizationCommand{
		Slug:                 r.Slug,
		ParentID:             r.ParentID,
		RegionCode:           r.RegionCode,
		DisplayName:          r.DisplayName,
		WebsiteURL:           r.WebsiteURL,
		LogoURL:              r.LogoURL,
		LogoDarkURL:          r.LogoDarkURL,
		FaviconURL:           r.FaviconURL,
		Status:               r.Status,
		Languages:            r.Languages,
		InitScore:            r.InitScore,
		DefaultApplicationID: r.DefaultApplicationID,

		DefaultAvatar:          r.DefaultAvatar,
		EnableDarkLogo:         r.EnableDarkLogo,
		PasswordType:           r.PasswordType,
		PasswordSalt:           r.PasswordSalt,
		PasswordObfuscatorType: r.PasswordObfuscatorType,
		PasswordObfuscatorKey:  r.PasswordObfuscatorKey,
		PasswordOptions:        r.PasswordOptions,
		DefaultPassword:        r.DefaultPassword,
		MasterPassword:         r.MasterPassword,
		MasterVerificationCode: r.MasterVerificationCode,
		DisableSignin:          r.DisableSignin,
		EnableTour:             r.EnableTour,
		HasPrivilegeConsent:    r.HasPrivilegeConsent,
		IsProfilePublic:        r.IsProfilePublic,
		UseEmailAsUsername:     r.UseEmailAsUsername,
	}
}

func (r PatchOrganizationRequest) toCommand(id string) apporganizations.PatchOrganizationCommand {
	return apporganizations.PatchOrganizationCommand{
		ID:                     id,
		ParentID:               r.ParentID,
		RegionCode:             r.RegionCode,
		DisplayName:            r.DisplayName,
		WebsiteURL:             r.WebsiteURL,
		LogoURL:                r.LogoURL,
		LogoDarkURL:            r.LogoDarkURL,
		FaviconURL:             r.FaviconURL,
		Status:                 r.Status,
		Languages:              r.Languages,
		InitScore:              r.InitScore,
		DefaultAvatar:          r.DefaultAvatar,
		PasswordObfuscatorType: r.PasswordObfuscatorType,
		DisableSignin:          r.DisableSignin,
		PasswordOptions:        r.PasswordOptions,
		EnableSoftDeletion:     r.EnableSoftDeletion,
		IpWhitelist:            r.IpWhitelist,
		InitiateLoginURL:       r.InitiateLoginURL,
		Balance:                r.Balance,
		Currency:               r.Currency,
		Revision:               r.Revision,
		// Phase 2: Extended fields
		PasswordType:           r.PasswordType,
		PasswordSalt:           r.PasswordSalt,
		PasswordObfuscatorKey:  r.PasswordObfuscatorKey,
		EnableDeposit:          r.EnableDeposit,
		BalanceCredit:          r.BalanceCredit,
		Mu:                     r.Mu,
		DefaultQuota:           r.DefaultQuota,
		DefaultCurrency:        r.DefaultCurrency,
		EnableDarkLogo:         r.EnableDarkLogo,
		HasPrivilegeConsent:    r.HasPrivilegeConsent,
		IsProfilePublic:        r.IsProfilePublic,
		UseEmailAsUsername:     r.UseEmailAsUsername,
		UsePermanentAvatar:     r.UsePermanentAvatar,
		EnableTour:             r.EnableTour,
		MasterPassword:         r.MasterPassword,
		MasterVerificationCode: r.MasterVerificationCode,
		DefaultPassword:        r.DefaultPassword,
		LdapAttributes:         r.LdapAttributes,
		KerberosRealm:          r.KerberosRealm,
		KerberosKdcHost:        r.KerberosKdcHost,
		KerberosKeytab:         r.KerberosKeytab,
		KerberosServiceName:    r.KerberosServiceName,
	}
}

func (r UpdateOrganizationSettingsRequest) toCommand(organizationID string) apporganizations.UpdateOrganizationSettingsCommand {
	accountItems := make([]OrganizationAccountItemRequest, 0)
	if r.AccountItems != nil {
		accountItems = r.AccountItems
	}

	converted := make([]apporganizations.AccountItemDTO, 0, len(accountItems))
	if r.AccountItems != nil {
		accountItems = r.AccountItems
	}

	for _, item := range accountItems {
		converted = append(converted, apporganizations.AccountItemDTO{
			Name:       item.Name,
			Visible:    item.Visible,
			ViewRule:   item.ViewRule,
			ModifyRule: item.ModifyRule,
			Regex:      item.Regex,
			Tab:        item.Tab,
		})
	}

	mfaItems := make([]apporganizations.MfaItemDTO, 0, len(r.MfaItems))
	for _, item := range r.MfaItems {
		mfaItems = append(mfaItems, apporganizations.MfaItemDTO{
			Name: item.Name,
			Rule: item.Rule,
		})
	}

	return apporganizations.UpdateOrganizationSettingsCommand{
		OrganizationID:       organizationID,
		Languages:            r.Languages,
		DefaultApplicationID: r.DefaultApplicationID,
		AccountItems:         converted,
		NavPreferences:       r.NavPreferences,
		FeatureFlags:         r.FeatureFlags,
		CountryCodes:         r.CountryCodes,
		UserTypes:            r.UserTypes,
		Tags:                 r.Tags,
		MfaItems:             mfaItems,
		AccountMenu:          r.AccountMenu,
		MfaRememberInHours:   r.MfaRememberInHours,
		DcrPolicy:            r.DcrPolicy,
	}
}

func (r UpdateOrganizationThemeRequest) toCommand(organizationID string) apporganizations.UpdateOrganizationThemeCommand {
	return apporganizations.UpdateOrganizationThemeCommand{
		OrganizationID: organizationID,
		ThemeName:      r.ThemeName,
		Tokens:         r.Tokens,
		CustomCSS:      r.CustomCSS,
		Assets:         r.Assets,
	}
}

func (r UpdateOrganizationSecurityPolicyRequest) toCommand(organizationID string) apporganizations.UpdateOrganizationSecurityPolicyCommand {
	return apporganizations.UpdateOrganizationSecurityPolicyCommand{
		OrganizationID:          organizationID,
		PasswordPolicy:          r.PasswordPolicy,
		PasswordDeriveAlgorithm: r.PasswordDeriveAlgorithm,
		MfaPolicy:               r.MfaPolicy,
		IPPolicy:                r.IPPolicy,
		SessionPolicy:           r.SessionPolicy,
		RiskPolicy:              r.RiskPolicy,
	}
}

type createOrganizationCommand struct {
	Slug       string
	ParentID   string
	RegionCode string
	// ActorID is the operator user id, filled by the handler from the session.
	ActorID              string
	DisplayName          string
	WebsiteURL           string
	LogoURL              string
	LogoDarkURL          string
	FaviconURL           string
	Status               string
	Languages            []string
	InitScore            int
	DefaultApplicationID string

	DefaultAvatar          string
	EnableDarkLogo         bool
	PasswordType           string
	PasswordSalt           string
	PasswordObfuscatorType string
	PasswordObfuscatorKey  string
	PasswordOptions        []string
	DefaultPassword        string
	MasterPassword         string
	MasterVerificationCode string
	DisableSignin          bool
	EnableTour             bool
	HasPrivilegeConsent    bool
	IsProfilePublic        bool
	UseEmailAsUsername     bool
}

// UpdateOrganizationConfigurationRequest bundles the four resource payloads the
// organization detail page edits. The server applies them in one transaction.
type UpdateOrganizationConfigurationRequest struct {
	Organization   PatchOrganizationRequest                `json:"organization"`
	Settings       UpdateOrganizationSettingsRequest       `json:"settings"`
	Theme          UpdateOrganizationThemeRequest          `json:"theme"`
	SecurityPolicy UpdateOrganizationSecurityPolicyRequest `json:"securityPolicy"`
}

func (r UpdateOrganizationConfigurationRequest) toCommand(id string) apporganizations.UpdateOrganizationConfigurationCommand {
	return apporganizations.UpdateOrganizationConfigurationCommand{
		Core:           r.Organization.toCommand(id),
		Settings:       r.Settings.toCommand(id),
		Theme:          r.Theme.toCommand(id),
		SecurityPolicy: r.SecurityPolicy.toCommand(id),
	}
}
