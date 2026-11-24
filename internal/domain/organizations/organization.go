package organizations

import (
	"fmt"
	"regexp"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// slugPattern validates slug format: lowercase letters, digits, and hyphens.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

type Organization struct {
	ID string
	// ParentID 是组织树的父节点（空 = 根）。树关系只影响控制面层级与策略继承，
	// 不改变数据面 organization_id 隔离（design/b2b）。
	ParentID    string
	Slug        string
	DisplayName string
	WebsiteURL  string
	LogoURL     string
	LogoDarkURL string
	FaviconURL  string
	Status      string
	// RegionCode 是组织所在的部署 region（数据驻留/路由的声明来源，空=未绑定）。
	RegionCode             string
	InitScore              int
	DefaultAvatar          string
	PasswordObfuscatorType string
	DisableSignin          bool
	DisableSignup          bool
	DefaultPassword        string
	PasswordOptions        []string
	EnableSoftDeletion     bool
	IpWhitelist            string
	InitiateLoginURL       string
	Balance                float64
	Currency               string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	DeletedAt              *time.Time
	CreatedBy              string
	UpdatedBy              string
	Revision               int64
	Settings               OrganizationSettings
	Theme                  OrganizationThemeSettings
	Security               OrganizationSecurityPolicy
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
	LdapAttributes         []string
	KerberosRealm          string
	KerberosKdcHost        string
	KerberosKeytab         string
	KerberosServiceName    string
}

// Validate checks the organization fields for business rule violations.
func (o *Organization) Validate() error {
	if o.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if o.Slug == "" {
		return apperrors.NewInvalidArgument("slug is required")
	}
	if len(o.Slug) < 2 || !slugPattern.MatchString(o.Slug) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("slug must be lowercase alphanumeric with hyphens: %s", o.Slug),
			map[string]any{"field": "slug", "value": o.Slug, "format": "slug"},
		)
	}
	if o.DisplayName == "" {
		return apperrors.NewInvalidArgument("displayName is required")
	}
	return nil
}

// IsSignupAllowed returns true if self-registration is enabled for this organization.
func (o *Organization) IsSignupAllowed() bool {
	return !o.DisableSignup
}

// IsSigninAllowed returns true if signin is enabled for this organization.
func (o *Organization) IsSigninAllowed() bool {
	return !o.DisableSignin
}
