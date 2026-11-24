package organizations

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	regions "github.com/guliangiam/guliangiam/internal/domain/regions"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   uow.IdentityUoW
	// RegionLister reads the deployment region registry for the organization
	// region picker. Nil keeps the endpoint empty (tests/non-region deployments).
	RegionLister regions.DeploymentRegionLister
	// PolicyInheritance reads declared inherit/override/lock rules for preview.
	PolicyInheritance orgdomain.PolicyInheritanceRepository
}

// 租户作用域说明：本包不做 RLS 提升。组织本体即租户边界：创建/更新/删除直接操作
// organizations 及其子表，列表返回范围内组织；写权限由 Casbin organizations:write 与
// RLS 决定，跨租户可见性来自上游 ScopeAuthorizer 放行后的作用域。
type CreateOrganizationCommand struct {
	Slug string
	// ParentID 是组织树父节点（空 = 根）；创建时按祖先链校验存在性/成环/深度。
	ParentID string
	// RegionCode 是组织所在部署 region（数据驻留/路由声明；空=未绑定）。
	RegionCode string
	// ActorID is the operator user id (empty for system actions).
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

	// Optional: branding
	DefaultAvatar  string
	EnableDarkLogo bool

	// Optional: password policy (empty = use default)
	PasswordType           string
	PasswordSalt           string
	PasswordObfuscatorType string
	PasswordObfuscatorKey  string
	PasswordOptions        []string
	DefaultPassword        string
	MasterPassword         string
	MasterVerificationCode string

	// Optional: security & signin
	DisableSignin       bool
	EnableTour          bool
	HasPrivilegeConsent bool
	IsProfilePublic     bool
	UseEmailAsUsername  bool
}

func (s *Service) NewOrganization(command CreateOrganizationCommand) (*orgdomain.Organization, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()

	// Password policy defaults (DB column_default 对齐)
	passwordType := command.PasswordType
	if passwordType == "" {
		passwordType = "argon2id"
	}
	passwordObfuscatorType := command.PasswordObfuscatorType
	if passwordObfuscatorType == "" {
		passwordObfuscatorType = "Plain"
	}
	passwordOptions := command.PasswordOptions
	if len(passwordOptions) == 0 {
		passwordOptions = []string{}
	}

	return &orgdomain.Organization{
		ID:          id,
		ParentID:    command.ParentID,
		RegionCode:  command.RegionCode,
		Slug:        command.Slug,
		DisplayName: command.DisplayName,
		WebsiteURL:  command.WebsiteURL,
		LogoURL:     command.LogoURL,
		LogoDarkURL: command.LogoDarkURL,
		FaviconURL:  command.FaviconURL,
		Status:      command.Status,
		InitScore:   command.InitScore,
		Settings: orgdomain.OrganizationSettings{
			OrganizationID:       id,
			Languages:            command.Languages,
			DefaultApplicationID: command.DefaultApplicationID,
		},

		// ── Optional overrides from command, else DB defaults ──
		DefaultAvatar:       command.DefaultAvatar,
		EnableDarkLogo:      command.EnableDarkLogo,
		DisableSignin:       command.DisableSignin,
		EnableTour:          command.EnableTour,
		HasPrivilegeConsent: command.HasPrivilegeConsent,
		IsProfilePublic:     command.IsProfilePublic,
		UseEmailAsUsername:  command.UseEmailAsUsername,

		// organization_auth_config
		PasswordType:           passwordType,
		PasswordSalt:           command.PasswordSalt,
		PasswordObfuscatorType: passwordObfuscatorType,
		PasswordObfuscatorKey:  command.PasswordObfuscatorKey,
		PasswordOptions:        passwordOptions,
		MasterPassword:         command.MasterPassword,
		MasterVerificationCode: command.MasterVerificationCode,
		DefaultPassword:        command.DefaultPassword,
		LdapAttributes:         []string{},

		// organization_kerberos_config
		KerberosRealm:       "",
		KerberosKdcHost:     "",
		KerberosKeytab:      "",
		KerberosServiceName: "HTTP",

		// organization_billing
		Currency:        "USD",
		DefaultCurrency: "CNY",

		// organization_branding: 全 bool，Go 零值 false = DB DEFAULT false ✓

		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
	}, nil
}
