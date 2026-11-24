package organizations

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func toOrganization(row organizationRow) *domain.Organization {
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
		PasswordOptions:        row.PasswordOptions,
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
		LdapAttributes:         row.LdapAttributes,
		KerberosRealm:          row.KerberosRealm,
		KerberosKdcHost:        row.KerberosKdcHost,
		KerberosKeytab:         row.KerberosKeytab,
		KerberosServiceName:    row.KerberosServiceName,
	}
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// organizationEncryptedMapper guards the DB-058 confidential columns:
// master_password, master_verification_code and default_password are stored as
// AES-256-GCM ciphertext.
var organizationEncryptedMapper = shared.NewEncryptedFieldMapper()

// organization_auth_config is keyed by organization_id, not by an "id" column.
const (
	organizationAuthConfigTable = "guliangiam.organization_auth_config"
	organizationPKColumn        = "organization_id"
)

func organizationAAD(organizationID, column string) crypto.AADContext {
	return crypto.AADContext{Table: organizationAuthConfigTable, Column: column, PKValue: organizationID}
}

// decryptOrganizationSecrets decrypts the confidential columns into the
// organization. It operates on a freshly built value, never on caller-owned
// state.
func decryptOrganizationSecrets(o *domain.Organization) error {
	if !organizationEncryptedMapper.IsEnabled() {
		return nil
	}
	masterPassword, err := organizationEncryptedMapper.DecryptColumn(organizationAAD(o.ID, "master_password"), o.MasterPassword)
	if err != nil {
		return apperrors.WrapInternalf(err, "decrypt organization master_password")
	}
	verificationCode, err := organizationEncryptedMapper.DecryptColumn(organizationAAD(o.ID, "master_verification_code"), o.MasterVerificationCode)
	if err != nil {
		return apperrors.WrapInternalf(err, "decrypt organization master_verification_code")
	}
	defaultPassword, err := organizationEncryptedMapper.DecryptColumn(organizationAAD(o.ID, "default_password"), o.DefaultPassword)
	if err != nil {
		return apperrors.WrapInternalf(err, "decrypt organization default_password")
	}
	o.MasterPassword = masterPassword
	o.MasterVerificationCode = verificationCode
	o.DefaultPassword = defaultPassword
	return nil
}

// organizationSecretsForWrite returns the values to persist for the confidential
// columns without mutating the domain object.
func organizationSecretsForWrite(o *domain.Organization) (masterPassword, masterVerificationCode, defaultPassword string, err error) {
	masterPassword, masterVerificationCode, defaultPassword = o.MasterPassword, o.MasterVerificationCode, o.DefaultPassword
	if !organizationEncryptedMapper.IsEnabled() {
		return masterPassword, masterVerificationCode, defaultPassword, nil
	}
	masterPassword, err = organizationEncryptedMapper.EncryptColumn(organizationAAD(o.ID, "master_password"), o.MasterPassword)
	if err != nil {
		return "", "", "", apperrors.WrapInternalf(err, "encrypt organization master_password")
	}
	masterVerificationCode, err = organizationEncryptedMapper.EncryptColumn(organizationAAD(o.ID, "master_verification_code"), o.MasterVerificationCode)
	if err != nil {
		return "", "", "", apperrors.WrapInternalf(err, "encrypt organization master_verification_code")
	}
	defaultPassword, err = organizationEncryptedMapper.EncryptColumn(organizationAAD(o.ID, "default_password"), o.DefaultPassword)
	if err != nil {
		return "", "", "", apperrors.WrapInternalf(err, "encrypt organization default_password")
	}
	return masterPassword, masterVerificationCode, defaultPassword, nil
}

func applySettings(target *domain.Organization, row organizationSettingsRow) error {
	settings := domain.OrganizationSettings{
		OrganizationID:       row.OrganizationID,
		DefaultApplicationID: row.DefaultApplicationID,
		AccountMenu:          row.AccountMenu,
		MfaRememberInHours:   row.MfaRememberInHours,
		DcrPolicy:            row.DcrPolicy,
	}

	if err := shared.UnmarshalJSON(row.Languages, &settings.Languages); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.AccountItems, &settings.AccountItems); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.NavPreferences, &settings.NavPreferences); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.FeatureFlags, &settings.FeatureFlags); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.CountryCodes, &settings.CountryCodes); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.UserTypes, &settings.UserTypes); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.Tags, &settings.Tags); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.MfaItems, &settings.MfaItems); err != nil {
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

	if err := shared.UnmarshalJSON(row.Tokens, &theme.Tokens); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.Assets, &theme.Assets); err != nil {
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

	if err := shared.UnmarshalJSON(row.PasswordPolicy, &security.PasswordPolicy); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.MfaPolicy, &security.MfaPolicy); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.IPPolicy, &security.IPPolicy); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.SessionPolicy, &security.SessionPolicy); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.RiskPolicy, &security.RiskPolicy); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.LockoutPolicy, &security.LockoutPolicy); err != nil {
		return err
	}
	if err := shared.UnmarshalJSON(row.ConcurrentSessionPolicy, &security.ConcurrentSessionPolicy); err != nil {
		return err
	}

	target.Security = security
	return nil
}
