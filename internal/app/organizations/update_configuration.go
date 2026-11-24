package organizations

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// UpdateOrganizationConfigurationCommand saves the four parts of an
// organization detail page (core row, settings, theme, security policy) in one
// database transaction. The existing per-resource endpoints stay for
// compatibility, but the console uses this command so a validation failure in
// any part cannot leave the others committed.
type UpdateOrganizationConfigurationCommand struct {
	Core           PatchOrganizationCommand
	Settings       UpdateOrganizationSettingsCommand
	Theme          UpdateOrganizationThemeCommand
	SecurityPolicy UpdateOrganizationSecurityPolicyCommand
}

func (s *Service) UpdateOrganizationConfiguration(ctx context.Context, command UpdateOrganizationConfigurationCommand) error {
	organizationID := strings.TrimSpace(command.Core.ID)
	if organizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	command.Settings.OrganizationID = organizationID
	command.Theme.OrganizationID = organizationID
	command.SecurityPolicy.OrganizationID = organizationID

	if err := validatePatchOrganizationCommand(command.Core); err != nil {
		return err
	}
	if err := validateUpdateOrganizationSettingsCommand(command.Settings); err != nil {
		return err
	}
	if err := validateUpdateOrganizationThemeCommand(command.Theme); err != nil {
		return err
	}
	if err := validateUpdateOrganizationSecurityPolicyCommand(command.SecurityPolicy); err != nil {
		return err
	}
	if err := s.ensureSecurityPolicyLocks(ctx, command.SecurityPolicy); err != nil {
		return err
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := s.patchOrganizationTx(ctx, tx, command.Core); err != nil {
			return err
		}
		if err := s.updateOrganizationSettingsTx(ctx, tx, command.Settings); err != nil {
			return err
		}
		if err := s.updateOrganizationThemeTx(ctx, tx, command.Theme); err != nil {
			return err
		}
		return s.updateOrganizationSecurityPolicyTx(ctx, tx, command.SecurityPolicy)
	})
}
