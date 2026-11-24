package organizations

import "time"

type organizationRow struct {
	ID                     string
	ParentID               *string
	Slug                   string
	DisplayName            string
	WebsiteURL             *string
	LogoURL                *string
	LogoDarkURL            *string
	FaviconURL             *string
	Status                 string
	RegionCode             string
	InitScore              int
	DefaultAvatar          *string
	PasswordObfuscatorType *string
	DisableSignin          bool
	PasswordOptions        []string
	EnableSoftDeletion     bool
	DisableSignup          bool
	IpWhitelist            string
	InitiateLoginURL       string
	Balance                float64
	Currency               string
	// Phase 2: Extended fields
	PasswordType           string
	PasswordSalt           string
	PasswordObfuscatorKey  string
	EnableDeposit          bool
	BalanceCredit          float64
	Mu                     int
	DefaultQuota           int
	DefaultCurrency        string
	EnableDarkLogo         bool
	HasPrivilegeConsent    bool
	IsProfilePublic        bool
	UseEmailAsUsername     bool
	UsePermanentAvatar     bool
	EnableTour             bool
	MasterPassword         string
	MasterVerificationCode string
	DefaultPassword        string
	LdapAttributes         []string
	KerberosRealm          string
	KerberosKdcHost        string
	KerberosKeytab         string
	KerberosServiceName    string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Revision               int64
	// DefaultApplicationID comes from the LEFT JOINed organization_settings row;
	// it is empty when the organization has no settings row (same as detail).
	DefaultApplicationID string
}

type organizationSettingsRow struct {
	OrganizationID       string
	Languages            []byte
	DefaultApplicationID string
	AccountItems         []byte
	NavPreferences       []byte
	FeatureFlags         []byte
	CountryCodes         []byte
	UserTypes            []byte
	Tags                 []byte
	DcrPolicy            string
	// Phase 2: Extended settings fields
	MfaItems           []byte
	AccountMenu        string
	MfaRememberInHours int
}

type organizationThemeRow struct {
	OrganizationID string
	ThemeName      string
	Tokens         []byte
	CustomCSS      string
	Assets         []byte
}

type organizationSecurityRow struct {
	OrganizationID          string
	PasswordPolicy          []byte
	PasswordDeriveAlgorithm string
	MfaPolicy               []byte
	IPPolicy                []byte
	SessionPolicy           []byte
	RiskPolicy              []byte
	LockoutPolicy           []byte
	ConcurrentSessionPolicy []byte
}
