package users

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	scopeauth "github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
)

type ImpersonateUserResult struct {
	Token string
}

// ImpersonateUser mints a 24h token for the target user. The target is resolved
// through the trusted tenant scope: {id} is attacker-controlled, so an
// unscoped GetByID would let a tenant admin impersonate any user id on MySQL
// (no RLS). callerUsername/callerOrgID still come from the session.
func (s *Service) ImpersonateUser(ctx context.Context, callerUsername, callerOrgID, organizationID, userID string) (ImpersonateUserResult, error) {
	// callerUsername/callerOrgID 来自会话，目标 userID 来自 URL path；impersonate 属 RiskCritical，由 ScopeAuthorizer 裁决、RLS 决定跨 org 目标可见性。
	result := ImpersonateUserResult{}
	if err := requireOrganizationID(organizationID); err != nil {
		return result, err
	}

	return result, runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Verify the caller is an admin.
		callerAdminID := ""
		var callerUUID, sourceTenantUUID, targetTenantUUID uuid.UUID
		if callerUsername != "" && callerOrgID != "" {
			caller, err := tx.Users().GetByUsername(ctx, callerOrgID, callerUsername)
			if err != nil {
				return apperrors.NewForbidden("caller not found")
			}
			if !caller.IsAdmin {
				return apperrors.NewForbidden("only admins can impersonate users")
			}
			callerAdminID = caller.ID
			callerUUID, _ = uuid.Parse(caller.ID)
			sourceTenantUUID, _ = uuid.Parse(caller.OrganizationID)
		}

		// Phase D-Step 3: impersonate RiskCritical 检查下沉到 Service 层
		// 不管是否跨租户，impersonate 都是最高风险操作 → 需要 approved ticket。
		if s.ScopeAuthorizer != nil && callerUUID != uuid.Nil {
			user, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
			if err != nil {
				return err
			}
			targetTenantUUID, _ = uuid.Parse(user.OrganizationID)

			// actorType 从 PrincipalResolver 查 — 但 ctx 已在 transport 层注入。
			// Service 层简化：先用 platform_admin（假设 admin 用户通常是 platform_admin）。
			// Phase D-Step 3 后续优化：从 SessionContext 读 PrincipalType。
			req := scopeauth.AuthorizationRequest{
				ActorID:      callerUUID,
				ActorType:    scopeauth.PrincipalPlatformAdmin,
				SourceTenant: sourceTenantUUID,
				TargetTenant: targetTenantUUID,
				ResourceType: "users",
				Action:       "impersonate",
				RiskLevel:    scopeauth.RiskCritical,
			}
			authResult, err := s.ScopeAuthorizer.Authorize(ctx, req)
			if err != nil {
				return fmt.Errorf("scope authorization failed: %w", err)
			}
			if !authResult.Allowed {
				return apperrors.NewForbidden(authResult.Reason)
			}
		}

		user, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
		if err != nil {
			return err
		}

		tokenID, err := s.UUID.NewString()
		if err != nil {
			return err
		}

		tokenValue := generateClientSecret()
		now := s.Clock.Now()
		expiresAt := now.Add(24 * time.Hour)

		// access_tokens.application_id 为 NOT NULL + FK：imp token 也必须绑定真实
		// 应用。优先目标用户的注册应用；无注册应用的用户回落其组织内任一应用；
		// 组织没有任何应用时明确拒绝，而不是让 NOT NULL 约束变成 500。
		applicationID := strings.TrimSpace(user.SignupApplicationID)
		if applicationID == "" {
			apps, _, err := tx.Applications().List(ctx, appdomain.ListQuery{
				OrganizationID: user.OrganizationID, Page: 1, PageSize: 1,
			})
			if err != nil {
				return err
			}
			if len(apps) == 0 {
				return apperrors.NewInvalidArgument("target user has no application binding for impersonation")
			}
			applicationID = apps[0].ID
		}

		metadata := map[string]any{
			"impersonated":    true,
			"impersonated_by": callerAdminID,
		}

		token := &tokendomain.Token{
			ID:             tokenID,
			UserID:         user.ID,
			Username:       user.Username,
			OrganizationID: user.OrganizationID,
			ApplicationID:  applicationID,
			TokenPrefix:    "imp",
			TokenHash:      platformcrypto.HashTokenValue(tokenValue),
			Scopes:         []string{"impersonate"},
			Audience:       []string{"api"},
			Metadata:       metadata,
			ExpiresAt:      expiresAt,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		if err := tx.Tokens().Create(ctx, token); err != nil {
			return apperrors.NewInternal("create impersonation token")
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			token.OrganizationID,
			"",
			"token.created",
			"Token",
			token.ID,
			"token",
			"token.created",
			map[string]any{},
		); err != nil {
			return err
		}

		result.Token = tokenValue
		return nil
	})
}
