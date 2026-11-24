package organizations

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PatchOrganizationCommand struct {
	ID                     string
	ParentID               *string
	RegionCode             *string
	DisplayName            *string
	WebsiteURL             *string
	LogoURL                *string
	LogoDarkURL            *string
	FaviconURL             *string
	Status                 *string
	Languages              []string
	InitScore              *int
	DefaultAvatar          *string
	PasswordObfuscatorType *string
	DisableSignin          *bool
	PasswordOptions        []string
	EnableSoftDeletion     *bool
	IpWhitelist            *string
	InitiateLoginURL       *string
	Balance                *float64
	Currency               *string
	Revision               int64
	// Phase 2: Extended fields
	PasswordType           *string
	PasswordSalt           *string
	PasswordObfuscatorKey  *string
	EnableDeposit          *bool
	BalanceCredit          *float64
	Mu                     *int
	DefaultQuota           *int
	DefaultCurrency        *string
	EnableDarkLogo         *bool
	HasPrivilegeConsent    *bool
	IsProfilePublic        *bool
	UseEmailAsUsername     *bool
	UsePermanentAvatar     *bool
	EnableTour             *bool
	MasterPassword         *string
	MasterVerificationCode *string
	DefaultPassword        *string
	LdapAttributes         []string
	KerberosRealm          *string
	KerberosKdcHost        *string
	KerberosKeytab         *string
	KerberosServiceName    *string
}

func (s *Service) PatchOrganization(ctx context.Context, command PatchOrganizationCommand) error {
	if err := validatePatchOrganizationCommand(command); err != nil {
		return err
	}

	// command.ID 来自 URL path；Update 按 id 执行，跨 org 能否命中 organizations 行由调用方作用域与 RLS USING/WITH CHECK 决定。
	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.patchOrganizationTx(ctx, tx, command)
	})
}

func (s *Service) patchOrganizationTx(ctx context.Context, tx appuow.UnitOfWork, command PatchOrganizationCommand) error {
	organization, err := tx.Organizations().GetByID(ctx, command.ID)
	if err != nil {
		return err
	}

	if command.Revision > 0 && organization.Revision != command.Revision {
		return apperrors.NewConflict("organization revision mismatch")
	}
	if command.RegionCode != nil {
		organization.RegionCode = *command.RegionCode
	}
	if command.ParentID != nil {
		if err := validateOrganizationParentChain(ctx, tx.Organizations(), organization.ID, *command.ParentID); err != nil {
			return err
		}
		organization.ParentID = *command.ParentID
	}

	if command.DisplayName != nil {
		organization.DisplayName = *command.DisplayName
	}
	if command.WebsiteURL != nil {
		organization.WebsiteURL = *command.WebsiteURL
	}
	if command.LogoURL != nil {
		organization.LogoURL = *command.LogoURL
	}
	if command.LogoDarkURL != nil {
		organization.LogoDarkURL = *command.LogoDarkURL
	}
	if command.FaviconURL != nil {
		organization.FaviconURL = *command.FaviconURL
	}
	if command.Status != nil {
		organization.Status = *command.Status
	}
	if command.Languages != nil {
		// languages live in organization_settings, not on the organizations row.
		// UpdateSettings upserts the whole settings row, so seed it with the
		// values already loaded by GetByID to avoid dropping the other keys.
		settings := organization.Settings
		settings.OrganizationID = organization.ID
		settings.Languages = command.Languages
		if err := tx.Organizations().UpdateSettings(ctx, &settings); err != nil {
			return err
		}
	}
	if command.InitScore != nil {
		organization.InitScore = *command.InitScore
	}
	if command.DefaultAvatar != nil {
		organization.DefaultAvatar = *command.DefaultAvatar
	}
	if command.PasswordObfuscatorType != nil {
		organization.PasswordObfuscatorType = *command.PasswordObfuscatorType
	}
	if command.DisableSignin != nil {
		organization.DisableSignin = *command.DisableSignin
	}
	if command.PasswordOptions != nil {
		organization.PasswordOptions = command.PasswordOptions
	}
	if command.EnableSoftDeletion != nil {
		organization.EnableSoftDeletion = *command.EnableSoftDeletion
	}
	if command.IpWhitelist != nil {
		organization.IpWhitelist = *command.IpWhitelist
	}
	if command.InitiateLoginURL != nil {
		organization.InitiateLoginURL = *command.InitiateLoginURL
	}
	if command.Balance != nil {
		organization.Balance = *command.Balance
	}
	if command.Currency != nil {
		organization.Currency = *command.Currency
	}

	// Phase 2: Extended field patching
	if command.PasswordType != nil {
		organization.PasswordType = *command.PasswordType
	}
	if command.PasswordSalt != nil {
		organization.PasswordSalt = *command.PasswordSalt
	}
	if command.PasswordObfuscatorKey != nil {
		organization.PasswordObfuscatorKey = *command.PasswordObfuscatorKey
	}
	if command.EnableDeposit != nil {
		organization.EnableDeposit = *command.EnableDeposit
	}
	if command.BalanceCredit != nil {
		organization.BalanceCredit = *command.BalanceCredit
	}
	if command.Mu != nil {
		organization.Mu = *command.Mu
	}
	if command.DefaultQuota != nil {
		organization.DefaultQuota = *command.DefaultQuota
	}
	if command.DefaultCurrency != nil {
		organization.DefaultCurrency = *command.DefaultCurrency
	}
	if command.EnableDarkLogo != nil {
		organization.EnableDarkLogo = *command.EnableDarkLogo
	}
	if command.HasPrivilegeConsent != nil {
		organization.HasPrivilegeConsent = *command.HasPrivilegeConsent
	}
	if command.IsProfilePublic != nil {
		organization.IsProfilePublic = *command.IsProfilePublic
	}
	if command.UseEmailAsUsername != nil {
		organization.UseEmailAsUsername = *command.UseEmailAsUsername
	}
	if command.UsePermanentAvatar != nil {
		organization.UsePermanentAvatar = *command.UsePermanentAvatar
	}
	if command.EnableTour != nil {
		organization.EnableTour = *command.EnableTour
	}
	if command.MasterPassword != nil {
		organization.MasterPassword = *command.MasterPassword
	}
	if command.MasterVerificationCode != nil {
		organization.MasterVerificationCode = *command.MasterVerificationCode
	}
	if command.DefaultPassword != nil {
		organization.DefaultPassword = *command.DefaultPassword
	}
	if command.LdapAttributes != nil {
		organization.LdapAttributes = command.LdapAttributes
	}
	if command.KerberosRealm != nil {
		organization.KerberosRealm = *command.KerberosRealm
	}
	if command.KerberosKdcHost != nil {
		organization.KerberosKdcHost = *command.KerberosKdcHost
	}
	if command.KerberosKeytab != nil {
		organization.KerberosKeytab = *command.KerberosKeytab
	}
	if command.KerberosServiceName != nil {
		organization.KerberosServiceName = *command.KerberosServiceName
	}

	organization.UpdatedAt = s.Clock.Now()
	organization.Revision++

	if err := tx.Organizations().Update(ctx, organization); err != nil {
		return err
	}

	if err := s.embedAuditEvent(
		tx,
		ctx,
		organization.ID,
		"",
		"organization.updated",
		"Organization",
		organization.ID,
		"Organization",
		"organization.updated",
		map[string]any{},
	); err != nil {
		return err
	}
	return nil
}
