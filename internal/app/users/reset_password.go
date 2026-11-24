package users

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/guliangiam/guliangiam/cred"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	trusteddevices "github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/passwordhistory"
)

const maxResetReasonRunes = 128

// ResetUserPasswordCommand 是管理员重置他人口令的唯一入参。
// ActorID/ActorIsPlatformAdmin/ActorOrganizationID 来自已认证会话，不接受请求体伪造。
type ResetUserPasswordCommand struct {
	OrganizationID string
	UserID         string
	Password       string
	Reason         string
	ActorID        string
	// ActorIsPlatformAdmin 由 handler 从 session.PrincipalType 判定；
	// 非平台管理员不得重置任何平台管理员账号。
	ActorIsPlatformAdmin bool
	ActorOrganizationID  string
}

type ResetUserPasswordResult struct {
	SessionsRevoked       int64
	RefreshRevoked        int64
	AccessRevoked         int64
	TrustedDevicesRevoked int64
}

// trustedDeviceRevoker 是 UoW 的窄能力断言（与 rememberme 等模块一致，
// 不在共享 UnitOfWork 接口上扩散可选仓储）。
type trustedDeviceRevoker interface {
	TrustedDevices() trusteddevices.Repository
}

// ResetUserPassword 重置目标用户口令：
//   - 目标必须在当前可信组织内（仓储显式 organization_id 过滤，跨组织 404）；
//   - 管理员不得重置自己（422，走自助路径）；
//   - 目标为平台管理员时：非平台管理员调用方 404；bootstrap 管理员即使由
//     平台管理员调用也 404；
//   - 口令必须通过组织 EffectivePasswordPolicy + 历史校验；
//   - 成功后置 need_update_password=true，并同事务吊销会话/令牌/受信设备。
func (s *Service) ResetUserPassword(ctx context.Context, command ResetUserPasswordCommand) (ResetUserPasswordResult, error) {
	result := ResetUserPasswordResult{}
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return result, err
	}
	if strings.TrimSpace(command.UserID) == "" {
		return result, apperrors.NewInvalidArgument("userId is required")
	}
	if command.Password == "" {
		return result, apperrors.NewInvalidArgument("password is required")
	}
	if utf8.RuneCountInString(command.Reason) > maxResetReasonRunes {
		return result, apperrors.NewUnprocessableEntity("reason must not exceed 128 characters")
	}

	credManager := cred.GetCredManager(userdomain.PasswordAlgorithmArgon2ID)
	if credManager == nil {
		return result, apperrors.NewInternal("argon2id credential manager unavailable")
	}
	now := s.Clock.Now().UTC()

	var targetUsername string
	err := runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
		if err != nil {
			return err
		}
		targetUsername = user.Username
		if command.ActorID != "" && command.ActorID == user.ID {
			return apperrors.NewUnprocessableEntity("administrators cannot reset their own password; use the self-service endpoint")
		}
		if err := assertCanManagePlatformAdmin(ctx, tx, command.ActorIsPlatformAdmin, user.ID); err != nil {
			return err
		}

		org, err := tx.Organizations().GetByID(ctx, command.OrganizationID)
		if err != nil {
			return err
		}
		policy := orgdomain.EffectivePasswordPolicy(org)
		if err := validatePasswordComplexity(command.Password, user.Username, policy); err != nil {
			return err
		}
		if policy.PasswordHistoryCount > 0 {
			history, herr := tx.Users().GetPasswordHistory(ctx, user.ID, policy.PasswordHistoryCount)
			if herr != nil {
				return herr
			}
			if err := checkPasswordHistoryAgainstEntries(history, command.Password, policy.PasswordHistoryCount); err != nil {
				return err
			}
		}

		// 旧明文不可得（管理员重置），因此记录“新口令”的历史哈希：
		// 它与校验使用同一算法，保证该口令在后续改密时不可复用。
		historyHash, hashErr := passwordhistory.Hash(command.Password)
		if hashErr != nil {
			return apperrors.NewInternal("hash password history")
		}
		historyID, idErr := s.UUID.NewString()
		if idErr != nil {
			return apperrors.NewInternal("generate password history id")
		}
		if err := tx.Users().CreatePasswordHistory(ctx, &userdomain.PasswordHistoryEntry{
			ID:           historyID,
			UserID:       user.ID,
			PasswordHash: historyHash,
			PasswordAlgo: userdomain.PasswordAlgorithmArgon2ID,
			CreatedAt:    now,
		}); err != nil {
			return err
		}

		newHash := credManager.GetHashedPassword(command.Password, "")
		if newHash == "" {
			return apperrors.NewInternal("hash user password")
		}
		if err := tx.Users().UpdatePasswordCredential(ctx, &userdomain.UserCredentials{
			UserID:             user.ID,
			HasPassword:        true,
			PasswordHash:       newHash,
			PasswordAlgo:       userdomain.PasswordAlgorithmArgon2ID,
			PasswordUpdatedAt:  &now,
			NeedUpdatePassword: true,
		}); err != nil {
			return err
		}

		user.UpdatedAt = now
		user.Revision++
		if err := tx.Users().UpdateForOrganization(ctx, user, command.OrganizationID); err != nil {
			return err
		}

		result.SessionsRevoked, err = tx.Sessions().RevokeByUser(ctx, user.ID, now)
		if err != nil {
			return err
		}
		result.RefreshRevoked, err = tx.Tokens().RevokeRefreshByUser(ctx, user.ID, now)
		if err != nil {
			return err
		}
		result.AccessRevoked, err = tx.Tokens().RevokeByQuery(ctx, tokendomain.RevokeQuery{UserID: user.ID}, now)
		if err != nil {
			return err
		}
		result.TrustedDevicesRevoked, err = revokeTrustedDevices(ctx, tx, user.OrganizationID, user.ID, now)
		if err != nil {
			return err
		}

		payload := map[string]any{
			"targetUserId":               user.ID,
			"sessionRevokedCount":        result.SessionsRevoked,
			"trustedDevicesRevokedCount": result.TrustedDevicesRevoked,
			"method":                     "admin",
			"actorOrganizationId":        command.ActorOrganizationID,
			"targetOrganizationId":       user.OrganizationID,
		}
		if command.Reason != "" {
			payload["reason"] = command.Reason
		}
		return s.embedAuditEvent(
			tx, ctx,
			user.OrganizationID, command.ActorID,
			"user.password.reset",
			"User", user.ID,
			"user", "user.password.reset",
			payload,
		)
	})
	if err != nil {
		return result, err
	}
	// 通知在事务提交后尽力发送：收件箱写入失败不回滚已完成的重置。
	s.notifyAdminPasswordReset(ctx, command.OrganizationID, command.UserID, targetUsername)
	return result, nil
}

// assertCanManagePlatformAdmin 保护平台级账号的破坏性管理操作（口令重置、
// 删除）：
//   - 非平台管理员调用方 → 404（不暴露目标存在性）；
//   - bootstrap 平台管理员 → 404（即使调用方是平台管理员）。
func assertCanManagePlatformAdmin(ctx context.Context, tx appuow.UnitOfWork, actorIsPlatformAdmin bool, targetUserID string) error {
	admin, err := tx.PlatformAdmins().GetByUserID(ctx, targetUserID)
	if err != nil {
		var appErr *apperrors.Error
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return nil
		}
		return err
	}
	if admin == nil || !admin.IsActive {
		return nil
	}
	if !actorIsPlatformAdmin {
		return apperrors.NewNotFound("user not found")
	}
	scopes, err := tx.PlatformAdminScopes().ListByPlatformAdmin(ctx, admin.ID, platformadmins.ScopeListOptions{})
	if err != nil {
		// 无法确认 bootstrap 标记时 fail closed，避免保护被绕过。
		return err
	}
	for _, scope := range scopes {
		if scope.IsBootstrap {
			return apperrors.NewNotFound("user not found")
		}
	}
	return nil
}

func revokeTrustedDevices(ctx context.Context, tx appuow.UnitOfWork, organizationID, userID string, now time.Time) (int64, error) {
	provider, ok := tx.(trustedDeviceRevoker)
	if !ok || provider.TrustedDevices() == nil {
		return 0, apperrors.NewInternal("trusted devices repository unavailable")
	}
	return provider.TrustedDevices().RevokeByUser(ctx, organizationID, userID, now)
}
