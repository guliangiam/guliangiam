package organizations

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
)

// AccountItemDTO decouples transport layer from domain AccountItem.
type AccountItemDTO struct {
	Name       string
	Visible    bool
	ViewRule   string
	ModifyRule string
	Regex      string
	Tab        string
}

// MfaItemDTO decouples transport layer from domain MfaItem.
type MfaItemDTO struct {
	Name string
	Rule string
}

func (a AccountItemDTO) toDomain() orgdomain.AccountItem {
	return orgdomain.AccountItem{
		Name: a.Name, Visible: a.Visible, ViewRule: a.ViewRule,
		ModifyRule: a.ModifyRule, Regex: a.Regex, Tab: a.Tab,
	}
}

func (m MfaItemDTO) toDomain() orgdomain.MfaItem {
	return orgdomain.MfaItem{Name: m.Name, Rule: m.Rule}
}

type UpdateOrganizationSettingsCommand struct {
	OrganizationID       string
	Languages            []string
	DefaultApplicationID string
	AccountItems         []AccountItemDTO
	NavPreferences       map[string]any
	FeatureFlags         map[string]any
	CountryCodes         []string
	UserTypes            []string
	Tags                 []string
	// Phase 2: Extended settings fields
	MfaItems           []MfaItemDTO
	AccountMenu        *string
	MfaRememberInHours *int
	DcrPolicy          *string
}

func (s *Service) UpdateOrganizationSettings(ctx context.Context, command UpdateOrganizationSettingsCommand) error {
	if err := validateUpdateOrganizationSettingsCommand(command); err != nil {
		return err
	}

	// settings 子表按目标 organization_id 写入；越权由上游放行的作用域与 RLS WITH CHECK 拦截。
	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.updateOrganizationSettingsTx(ctx, tx, command)
	})
}

func (s *Service) updateOrganizationSettingsTx(ctx context.Context, tx appuow.UnitOfWork, command UpdateOrganizationSettingsCommand) error {
	if err := s.ensurePolicyLocks(ctx, command.OrganizationID, []string{"accountItems", "mfaItems", "languages", "featureFlags"}); err != nil {
		return err
	}
	// 校验目标组织存在；跨 org 可见性由上游 ScopeAuthorizer 放行的作用域与 RLS 决定。
	if _, err := tx.Organizations().GetByID(ctx, command.OrganizationID); err != nil {
		return err
	}

	accountItems := make([]orgdomain.AccountItem, len(command.AccountItems))
	for i, a := range command.AccountItems {
		accountItems[i] = a.toDomain()
	}
	mfaItems := make([]orgdomain.MfaItem, len(command.MfaItems))
	for i, m := range command.MfaItems {
		mfaItems[i] = m.toDomain()
	}

	settings := orgdomain.OrganizationSettings{
		OrganizationID:       command.OrganizationID,
		Languages:            command.Languages,
		DefaultApplicationID: command.DefaultApplicationID,
		AccountItems:         accountItems,
		NavPreferences:       command.NavPreferences,
		FeatureFlags:         command.FeatureFlags,
		CountryCodes:         command.CountryCodes,
		UserTypes:            command.UserTypes,
		Tags:                 command.Tags,
		MfaItems:             mfaItems,
	}
	if command.AccountMenu != nil {
		settings.AccountMenu = *command.AccountMenu
	}
	if command.MfaRememberInHours != nil {
		settings.MfaRememberInHours = *command.MfaRememberInHours
	}
	if command.DcrPolicy != nil {
		settings.DcrPolicy = *command.DcrPolicy
	}

	// ⚠️ 不调 repo.Update 主表：主表 revision 由 PATCH /organizations/:id 单点管理。
	// 前端 Promise.all 并行 settings/theme/security-policy 三个端点时，
	// 如果每个都 CAS 主表 revision 会形成竞态——只有一个能成功，其他 Conflict。
	// 子表端点只写自己对应的子表。
	if err := tx.Organizations().UpdateSettings(ctx, &settings); err != nil {
		return err
	}

	return s.embedAuditEvent(
		tx,
		ctx,
		command.OrganizationID,
		"",
		"org.settings.updated",
		"organization",
		command.OrganizationID,
		"organization",
		"org.settings.updated",
		map[string]any{keyOrgID: command.OrganizationID},
	)
}
