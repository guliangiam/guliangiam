package organizations

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type UpdateOrganizationSecurityPolicyCommand struct {
	OrganizationID          string
	PasswordPolicy          map[string]any
	PasswordDeriveAlgorithm string
	MfaPolicy               map[string]any
	IPPolicy                map[string]any
	SessionPolicy           map[string]any
	RiskPolicy              map[string]any
}

func (s *Service) UpdateOrganizationSecurityPolicy(ctx context.Context, command UpdateOrganizationSecurityPolicyCommand) error {
	if err := validateUpdateOrganizationSecurityPolicyCommand(command); err != nil {
		return err
	}
	if err := s.ensureSecurityPolicyLocks(ctx, command); err != nil {
		return err
	}

	// command.OrganizationID 来自 URL path：先校验目标组织存在，再 upsert organization_security_policies；子表无 org 条件，跨 org 能否命中由调用方作用域与 RLS 决定。
	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.updateOrganizationSecurityPolicyTx(ctx, tx, command)
	})
}

func (s *Service) updateOrganizationSecurityPolicyTx(ctx context.Context, tx appuow.UnitOfWork, command UpdateOrganizationSecurityPolicyCommand) error {
	organization, err := tx.Organizations().GetByID(ctx, command.OrganizationID)
	if err != nil {
		return err
	}

	security := orgdomain.OrganizationSecurityPolicy{
		OrganizationID:          command.OrganizationID,
		PasswordPolicy:          command.PasswordPolicy,
		PasswordDeriveAlgorithm: command.PasswordDeriveAlgorithm,
		MfaPolicy:               command.MfaPolicy,
		IPPolicy:                command.IPPolicy,
		SessionPolicy:           command.SessionPolicy,
		RiskPolicy:              command.RiskPolicy,
		HasMasterKey:            organization.Security.HasMasterKey, // 保留主表已有值，不前端透传
	}

	// ⚠️ 不调 repo.Update 主表：主表 revision 由 PATCH /organizations/:id 单点管理。
	// 前端 Promise.all 并行 settings/theme/security-policy 三个端点时，
	// 如果每个都 CAS 主表 revision 会形成竞态——只有一个能成功，其他 Conflict。
	// 子表端点只写自己对应的子表。
	if err := tx.Organizations().UpdateSecurityPolicy(ctx, &security); err != nil {
		return err
	}

	return s.embedAuditEvent(
		tx,
		ctx,
		command.OrganizationID,
		"",
		"org.security_policy.updated",
		"organization",
		command.OrganizationID,
		"organization",
		"org.security_policy.updated",
		map[string]any{keyOrgID: command.OrganizationID},
	)
}

// ensureSecurityPolicyLocks rejects writes to a key whose ancestor declared
// mode=lock (design/b2b §3, AC-B2B-001). The target organization may update its
// own lock declaration; only an ancestor lock is a hard 403.
func (s *Service) ensureSecurityPolicyLocks(ctx context.Context, command UpdateOrganizationSecurityPolicyCommand) error {
	keys := make([]string, 0, 6)
	if command.PasswordPolicy != nil {
		keys = append(keys, "passwordPolicy")
	}
	if command.MfaPolicy != nil {
		keys = append(keys, "mfaPolicy")
	}
	if command.IPPolicy != nil {
		keys = append(keys, "ipPolicy")
	}
	if command.SessionPolicy != nil {
		keys = append(keys, "sessionPolicy")
	}
	if command.RiskPolicy != nil {
		keys = append(keys, "riskPolicy")
	}
	if command.PasswordDeriveAlgorithm != "" {
		keys = append(keys, "passwordDeriveAlgorithm")
	}
	return s.ensurePolicyLocks(ctx, command.OrganizationID, keys)
}

// ensurePolicyLocks is shared by security/settings/theme writes.
func (s *Service) ensurePolicyLocks(ctx context.Context, organizationID string, keys []string) error {
	if s.PolicyInheritance == nil {
		return nil
	}
	for _, key := range keys {
		preview, err := s.PreviewPolicyInheritance(ctx, organizationID, key)
		if err != nil {
			return err
		}
		if preview.Locked && preview.LockedByOrganizationID != "" && preview.LockedByOrganizationID != organizationID {
			return apperrors.NewForbiddenf("policy %s is locked by ancestor organization %s", key, preview.LockedByOrganizationID)
		}
	}
	return nil
}
