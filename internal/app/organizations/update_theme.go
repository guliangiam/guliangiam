package organizations

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
)

type UpdateOrganizationThemeCommand struct {
	OrganizationID string
	ThemeName      string
	Tokens         map[string]any
	CustomCSS      string
	Assets         map[string]any
}

func (s *Service) UpdateOrganizationTheme(ctx context.Context, command UpdateOrganizationThemeCommand) error {
	if err := validateUpdateOrganizationThemeCommand(command); err != nil {
		return err
	}

	// command.OrganizationID 来自 URL path：先校验目标组织存在，再 upsert organization_theme_settings；子表无 org 条件，跨 org 能否命中由调用方作用域与 RLS 决定。
	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.updateOrganizationThemeTx(ctx, tx, command)
	})
}

func (s *Service) updateOrganizationThemeTx(ctx context.Context, tx appuow.UnitOfWork, command UpdateOrganizationThemeCommand) error {
	if err := s.ensurePolicyLocks(ctx, command.OrganizationID, []string{"theme"}); err != nil {
		return err
	}
	if _, err := tx.Organizations().GetByID(ctx, command.OrganizationID); err != nil {
		return err
	}

	theme := orgdomain.OrganizationThemeSettings{
		OrganizationID: command.OrganizationID,
		ThemeName:      command.ThemeName,
		Tokens:         command.Tokens,
		CustomCSS:      command.CustomCSS,
		Assets:         command.Assets,
	}

	// ⚠️ 不调 repo.Update 主表：主表 revision 由 PATCH /organizations/:id 单点管理。
	// 前端 Promise.all 并行 settings/theme/security-policy 三个端点时，
	// 如果每个都 CAS 主表 revision 会形成竞态——只有一个能成功，其他 Conflict。
	// 子表端点只写自己对应的子表。
	if err := tx.Organizations().UpdateTheme(ctx, &theme); err != nil {
		return err
	}

	return s.embedAuditEvent(
		tx,
		ctx,
		command.OrganizationID,
		"",
		"org.theme.updated",
		"organization",
		command.OrganizationID,
		"organization",
		"org.theme.updated",
		map[string]any{keyOrgID: command.OrganizationID, "themeName": command.ThemeName},
	)
}
