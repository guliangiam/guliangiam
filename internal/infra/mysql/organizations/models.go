package organizations

import "time"

// organizationRow 是组织主表与 auth/kerberos/billing/branding 子表的逻辑基础行。
// 与 PG 版 internal/infra/postgres/organizations/models.go 同构，
// 但 password_options / ldap_attributes 从 PG TEXT[] 的 []string 改为
// MySQL JSON 的 []byte，mapper 层做 JSON → []string 转换。
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
	PasswordOptions        []byte // JSON
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
	LdapAttributes         []byte // JSON
	KerberosRealm          string
	KerberosKdcHost        string
	KerberosKeytab         string
	KerberosServiceName    string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Revision               int64
}

// organizationSettingsRow 是 organization_settings 表的扫描目标。
// LEFT JOIN 时所有列都可能为 NULL，repository 层扫描时用指针承接。
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

// organizationThemeRow 是 organization_theme_settings 表的扫描目标。
type organizationThemeRow struct {
	OrganizationID string
	ThemeName      string
	Tokens         []byte
	CustomCSS      string
	Assets         []byte
}

// organizationSecurityRow 是 organization_security_policies 表的扫描目标。
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

// aggregateRow 承接 JOIN 查询的全部列：
//   - base organization (44 列)
//   - settings (13 列，LEFT JOIN 可 NULL)
//   - theme (5 列，LEFT JOIN 可 NULL)
//   - security (8 列，LEFT JOIN 可 NULL)
type aggregateRow struct {
	base     organizationRow
	settings organizationSettingsRow
	hasSet   bool
	theme    organizationThemeRow
	hasTheme bool
	security organizationSecurityRow
	hasSec   bool
}
