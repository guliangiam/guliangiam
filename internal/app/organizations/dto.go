package organizations

import (
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
)

type OrganizationListItem struct {
	ID                   string    `json:"id"`
	ParentID             string    `json:"parentId"`
	RegionCode           string    `json:"regionCode"`
	Slug                 string    `json:"slug"`
	DisplayName          string    `json:"displayName"`
	WebsiteURL           string    `json:"websiteUrl"`
	LogoURL              string    `json:"logoUrl"`
	DefaultApplicationID string    `json:"defaultApplicationId"`
	Status               string    `json:"status"`
	PasswordType         string    `json:"passwordType"`
	Balance              float64   `json:"balance"`
	Currency             string    `json:"currency"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

type OrganizationDetail struct {
	OrganizationListItem
	LogoDarkURL            string         `json:"logoDarkUrl"`
	FaviconURL             string         `json:"faviconUrl"`
	InitScore              int            `json:"initScore"`
	DefaultAvatar          string         `json:"defaultAvatar"`
	PasswordObfuscatorType string         `json:"passwordObfuscatorType"`
	DisableSignin          bool           `json:"disableSignin"`
	PasswordOptions        []string       `json:"passwordOptions"`
	EnableSoftDeletion     bool           `json:"enableSoftDeletion"`
	IpWhitelist            string         `json:"ipWhitelist"`
	InitiateLoginURL       string         `json:"initiateLoginUrl"`
	Settings               map[string]any `json:"settings"`
	Theme                  map[string]any `json:"theme"`
	SecurityPolicySummary  map[string]any `json:"securityPolicySummary"`
	Revision               int64          `json:"revision"`
	// Phase 2: Extended fields
	PasswordType string `json:"passwordType"`
	// Credential columns are write-only through the management API: the read
	// projection carries an is-set boolean plus a masked display string, never
	// the stored value (same shape as certs/providers, enforced by
	// scripts/check-credential-masking.py). Create/Patch still accept plaintext.
	PasswordSaltSet              bool     `json:"passwordSaltSet"`
	PasswordSaltMasked           string   `json:"passwordSaltMasked,omitempty"`
	PasswordObfuscatorKeySet     bool     `json:"passwordObfuscatorKeySet"`
	PasswordObfuscatorKeyMasked  string   `json:"passwordObfuscatorKeyMasked,omitempty"`
	EnableDeposit                bool     `json:"enableDeposit"`
	BalanceCredit                float64  `json:"balanceCredit"`
	Mu                           int      `json:"mu"`
	DefaultQuota                 int      `json:"defaultQuota"`
	DefaultCurrency              string   `json:"defaultCurrency"`
	EnableDarkLogo               bool     `json:"enableDarkLogo"`
	HasPrivilegeConsent          bool     `json:"hasPrivilegeConsent"`
	IsProfilePublic              bool     `json:"isProfilePublic"`
	UseEmailAsUsername           bool     `json:"useEmailAsUsername"`
	UsePermanentAvatar           bool     `json:"usePermanentAvatar"`
	EnableTour                   bool     `json:"enableTour"`
	MasterPasswordSet            bool     `json:"masterPasswordSet"`
	MasterPasswordMasked         string   `json:"masterPasswordMasked,omitempty"`
	MasterVerificationCodeSet    bool     `json:"masterVerificationCodeSet"`
	MasterVerificationCodeMasked string   `json:"masterVerificationCodeMasked,omitempty"`
	DefaultPasswordSet           bool     `json:"defaultPasswordSet"`
	DefaultPasswordMasked        string   `json:"defaultPasswordMasked,omitempty"`
	LdapAttributes               []string `json:"ldapAttributes"`
	KerberosRealm                string   `json:"kerberosRealm"`
	KerberosKdcHost              string   `json:"kerberosKdcHost"`
	KerberosKeytabSet            bool     `json:"kerberosKeytabSet"`
	KerberosKeytabMasked         string   `json:"kerberosKeytabMasked,omitempty"`
	KerberosServiceName          string   `json:"kerberosServiceName"`
}

// OrganizationPublicDetail is the pre-login discovery projection served by the
// anonymous GET /api/v1/organizations/by-slug/{slug} and
// GET /api/v1/organizations/resolve endpoints (see auth_paths.go). It carries
// branding, theme and the default application only.
//
// Credential-bearing columns (masterPassword, masterVerificationCode,
// defaultPassword, passwordSalt, passwordObfuscatorKey, kerberosKeytab) must
// never be added to this struct: an absent field is the fix, blanking a value
// is not. The management surfaces keep using OrganizationDetail.
type OrganizationPublicDetail struct {
	ID                   string         `json:"id"`
	Slug                 string         `json:"slug"`
	DisplayName          string         `json:"displayName"`
	WebsiteURL           string         `json:"websiteUrl"`
	LogoURL              string         `json:"logoUrl"`
	LogoDarkURL          string         `json:"logoDarkUrl"`
	FaviconURL           string         `json:"faviconUrl"`
	DefaultAvatar        string         `json:"defaultAvatar"`
	DefaultApplicationID string         `json:"defaultApplicationId"`
	Status               string         `json:"status"`
	PasswordType         string         `json:"passwordType"`
	DisableSignin        bool           `json:"disableSignin"`
	InitiateLoginURL     string         `json:"initiateLoginUrl"`
	Theme                map[string]any `json:"theme"`
	CreatedAt            time.Time      `json:"createdAt"`
	UpdatedAt            time.Time      `json:"updatedAt"`
}

// toPublicDetail maps the organization domain aggregate onto the minimal
// public projection. It intentionally reads no credential field.
func toPublicDetail(organization *orgdomain.Organization) OrganizationPublicDetail {
	return OrganizationPublicDetail{
		ID:                   organization.ID,
		Slug:                 organization.Slug,
		DisplayName:          organization.DisplayName,
		WebsiteURL:           organization.WebsiteURL,
		LogoURL:              organization.LogoURL,
		LogoDarkURL:          organization.LogoDarkURL,
		FaviconURL:           organization.FaviconURL,
		DefaultAvatar:        organization.DefaultAvatar,
		DefaultApplicationID: organization.Settings.DefaultApplicationID,
		Status:               organization.Status,
		PasswordType:         organization.PasswordType,
		DisableSignin:        organization.DisableSignin,
		InitiateLoginURL:     organization.InitiateLoginURL,
		Theme: map[string]any{
			"themeName": organization.Theme.ThemeName,
			"tokens":    organization.Theme.Tokens,
			"customCss": organization.Theme.CustomCSS,
			"assets":    organization.Theme.Assets,
		},
		CreatedAt: organization.CreatedAt,
		UpdatedAt: organization.UpdatedAt,
	}
}
