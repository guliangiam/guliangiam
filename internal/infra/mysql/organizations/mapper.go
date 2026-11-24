package organizations

import (
	"encoding/json"

	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/organizations"
)

// orgEncMapper 负责 master_password / master_verification_code /
// default_password 三列的 AES-256-GCM 加解密（DB-058 机密列）。
// MySQL 复用同一 AAD 口径（organization_auth_config，PK organization_id），
// 与 PG 版写入的密文语义一致。
var orgEncMapper = mysqlshared.NewEncryptedFieldMapper()

const (
	organizationAuthConfigTable = "guliangiam.organization_auth_config"
)

func organizationAAD(organizationID, column string) crypto.AADContext {
	return crypto.AADContext{Table: organizationAuthConfigTable, Column: column, PKValue: organizationID}
}

// derefString 把 *string 安全转成 string，nil 时返回 ""。
func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// derefBytes 把 *[]byte 安全转成 []byte，nil 时返回 nil。
func derefBytes(p *[]byte) []byte {
	if p == nil {
		return nil
	}
	return *p
}

// derefInt 把 *int 安全转成 int，nil 时返回 0。
func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// toOrganization 把 MySQL 表扫描结果转成 domain.Organization。
// PasswordOptions / LdapAttributes 从 JSON []byte 反序列化为 []string。
func toOrganization(row organizationRow) (*domain.Organization, error) {
	var passwordOptions []string
	if len(row.PasswordOptions) > 0 {
		if err := json.Unmarshal(row.PasswordOptions, &passwordOptions); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal password_options")
		}
	}

	var ldapAttributes []string
	if len(row.LdapAttributes) > 0 {
		if err := json.Unmarshal(row.LdapAttributes, &ldapAttributes); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal ldap_attributes")
		}
	}

	return &domain.Organization{
		ID:                     row.ID,
		ParentID:               derefString(row.ParentID),
		Slug:                   row.Slug,
		DisplayName:            row.DisplayName,
		WebsiteURL:             derefString(row.WebsiteURL),
		LogoURL:                derefString(row.LogoURL),
		LogoDarkURL:            derefString(row.LogoDarkURL),
		FaviconURL:             derefString(row.FaviconURL),
		Status:                 row.Status,
		RegionCode:             row.RegionCode,
		InitScore:              row.InitScore,
		DefaultAvatar:          derefString(row.DefaultAvatar),
		PasswordObfuscatorType: derefString(row.PasswordObfuscatorType),
		DisableSignin:          row.DisableSignin,
		PasswordOptions:        passwordOptions,
		EnableSoftDeletion:     row.EnableSoftDeletion,
		DisableSignup:          row.DisableSignup,
		IpWhitelist:            row.IpWhitelist,
		InitiateLoginURL:       row.InitiateLoginURL,
		Balance:                row.Balance,
		Currency:               row.Currency,
		CreatedAt:              row.CreatedAt,
		UpdatedAt:              row.UpdatedAt,
		Revision:               row.Revision,
		// Phase 2: Extended fields
		PasswordType:           row.PasswordType,
		PasswordSalt:           row.PasswordSalt,
		PasswordObfuscatorKey:  row.PasswordObfuscatorKey,
		EnableDeposit:          row.EnableDeposit,
		BalanceCredit:          row.BalanceCredit,
		Mu:                     row.Mu,
		DefaultQuota:           row.DefaultQuota,
		DefaultCurrency:        row.DefaultCurrency,
		EnableDarkLogo:         row.EnableDarkLogo,
		HasPrivilegeConsent:    row.HasPrivilegeConsent,
		IsProfilePublic:        row.IsProfilePublic,
		UseEmailAsUsername:     row.UseEmailAsUsername,
		UsePermanentAvatar:     row.UsePermanentAvatar,
		EnableTour:             row.EnableTour,
		MasterPassword:         row.MasterPassword,
		MasterVerificationCode: row.MasterVerificationCode,
		DefaultPassword:        row.DefaultPassword,
		LdapAttributes:         ldapAttributes,
		KerberosRealm:          row.KerberosRealm,
		KerberosKdcHost:        row.KerberosKdcHost,
		KerberosKeytab:         row.KerberosKeytab,
		KerberosServiceName:    row.KerberosServiceName,
	}, nil
}

// decryptOrganizationSecrets 解密机密列（master_password 等），
// 对返回值操作，不改调用方。
func decryptOrganizationSecrets(o *domain.Organization) error {
	if !orgEncMapper.IsEnabled() {
		return nil
	}
	masterPassword, err := orgEncMapper.DecryptColumn(organizationAAD(o.ID, "master_password"), o.MasterPassword)
	if err != nil {
		return apperrors.WrapInternalf(err, "decrypt organization master_password")
	}
	verificationCode, err := orgEncMapper.DecryptColumn(organizationAAD(o.ID, "master_verification_code"), o.MasterVerificationCode)
	if err != nil {
		return apperrors.WrapInternalf(err, "decrypt organization master_verification_code")
	}
	defaultPassword, err := orgEncMapper.DecryptColumn(organizationAAD(o.ID, "default_password"), o.DefaultPassword)
	if err != nil {
		return apperrors.WrapInternalf(err, "decrypt organization default_password")
	}
	o.MasterPassword = masterPassword
	o.MasterVerificationCode = verificationCode
	o.DefaultPassword = defaultPassword
	return nil
}

// organizationSecretsForWrite 返回持久化机密列所需的值，不改 domain 对象。
func organizationSecretsForWrite(o *domain.Organization) (masterPassword, masterVerificationCode, defaultPassword string, err error) {
	masterPassword, masterVerificationCode, defaultPassword = o.MasterPassword, o.MasterVerificationCode, o.DefaultPassword
	if !orgEncMapper.IsEnabled() {
		return masterPassword, masterVerificationCode, defaultPassword, nil
	}
	masterPassword, err = orgEncMapper.EncryptColumn(organizationAAD(o.ID, "master_password"), o.MasterPassword)
	if err != nil {
		return "", "", "", apperrors.WrapInternalf(err, "encrypt organization master_password")
	}
	masterVerificationCode, err = orgEncMapper.EncryptColumn(organizationAAD(o.ID, "master_verification_code"), o.MasterVerificationCode)
	if err != nil {
		return "", "", "", apperrors.WrapInternalf(err, "encrypt organization master_verification_code")
	}
	defaultPassword, err = orgEncMapper.EncryptColumn(organizationAAD(o.ID, "default_password"), o.DefaultPassword)
	if err != nil {
		return "", "", "", apperrors.WrapInternalf(err, "encrypt organization default_password")
	}
	return masterPassword, masterVerificationCode, defaultPassword, nil
}

// marshalPasswordOptions 把 []string 序列化成 JSON，供 INSERT/UPDATE 使用。
func marshalPasswordOptions(opts []string) ([]byte, error) {
	if opts == nil {
		opts = []string{}
	}
	return json.Marshal(opts)
}

// marshalLdapAttributes 把 []string 序列化成 JSON，供 INSERT/UPDATE 使用。
func marshalLdapAttributes(attrs []string) ([]byte, error) {
	if attrs == nil {
		attrs = []string{}
	}
	return json.Marshal(attrs)
}

func applySettings(target *domain.Organization, row organizationSettingsRow) error {
	settings := domain.OrganizationSettings{
		OrganizationID:       row.OrganizationID,
		DefaultApplicationID: row.DefaultApplicationID,
		AccountMenu:          row.AccountMenu,
		MfaRememberInHours:   row.MfaRememberInHours,
		DcrPolicy:            row.DcrPolicy,
	}

	if err := mysqlshared.UnmarshalJSON(row.Languages, &settings.Languages); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.AccountItems, &settings.AccountItems); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.NavPreferences, &settings.NavPreferences); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.FeatureFlags, &settings.FeatureFlags); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.CountryCodes, &settings.CountryCodes); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.UserTypes, &settings.UserTypes); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.Tags, &settings.Tags); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.MfaItems, &settings.MfaItems); err != nil {
		return err
	}

	target.Settings = settings
	return nil
}

func applyTheme(target *domain.Organization, row organizationThemeRow) error {
	theme := domain.OrganizationThemeSettings{
		OrganizationID: row.OrganizationID,
		ThemeName:      row.ThemeName,
		CustomCSS:      row.CustomCSS,
	}

	if err := mysqlshared.UnmarshalJSON(row.Tokens, &theme.Tokens); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.Assets, &theme.Assets); err != nil {
		return err
	}

	target.Theme = theme
	return nil
}

func applySecurity(target *domain.Organization, row organizationSecurityRow) error {
	security := domain.OrganizationSecurityPolicy{
		OrganizationID:          row.OrganizationID,
		PasswordDeriveAlgorithm: row.PasswordDeriveAlgorithm,
	}
	if security.PasswordDeriveAlgorithm == "" {
		security.PasswordDeriveAlgorithm = domain.PasswordDeriveAlgorithmHMACSHA256
	}

	if err := mysqlshared.UnmarshalJSON(row.PasswordPolicy, &security.PasswordPolicy); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.MfaPolicy, &security.MfaPolicy); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.IPPolicy, &security.IPPolicy); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.SessionPolicy, &security.SessionPolicy); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.RiskPolicy, &security.RiskPolicy); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.LockoutPolicy, &security.LockoutPolicy); err != nil {
		return err
	}
	if err := mysqlshared.UnmarshalJSON(row.ConcurrentSessionPolicy, &security.ConcurrentSessionPolicy); err != nil {
		return err
	}

	target.Security = security
	return nil
}
