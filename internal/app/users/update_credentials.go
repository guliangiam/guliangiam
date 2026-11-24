package users

import (
	"context"

	"github.com/guliangiam/guliangiam/cred"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type UpdateUserCredentialsCommand struct {
	UserID             string
	OrganizationID     string // Required for password policy validation
	Username           string // Required for password policy validation
	Password           string
	RotateAccessSecret bool
	// ActorID 是已认证调用方的 userID，写入审计归属；内部系统调用可为空。
	ActorID string
	// NeedUpdatePassword 为 nil 表示「不修改」，显式 false 表示清除标记。
	// 用指针区分「未传」与「传了 false」，否则无法把已置位的标记改回 false。
	NeedUpdatePassword *bool
}

type UpdateUserCredentialsResult struct {
	RotatedAccessKey    string
	RotatedAccessSecret string
}

// loadUserForCredentials keeps the request-tenant lookup when a tenant is
// supplied, and falls back to the system-scoped lookup for internal callers
// that operate without a request scope.
func loadUserForCredentials(ctx context.Context, tx appuow.UnitOfWork, userID, organizationID string) (*userdomain.User, error) {
	if organizationID == "" {
		return tx.Users().GetByID(ctx, userID)
	}
	return tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
}

func persistUserForCredentials(ctx context.Context, tx appuow.UnitOfWork, user *userdomain.User, organizationID string) error {
	if organizationID == "" {
		return tx.Users().Update(ctx, user)
	}
	return tx.Users().UpdateForOrganization(ctx, user, organizationID)
}

func (s *Service) UpdateUserCredentials(ctx context.Context, command UpdateUserCredentialsCommand) (UpdateUserCredentialsResult, error) {
	returnResult := UpdateUserCredentialsResult{}
	// Management routes always pass the trusted tenant. Internal password
	// rotation helpers (WebAuthn/access-key flows) still call this use case
	// without one and keep the system-scoped lookup.
	scopedOrganizationID := command.OrganizationID
	if scopedOrganizationID != "" {
		if err := requireOrganizationID(scopedOrganizationID); err != nil {
			return returnResult, err
		}
	}
	if err := validateUpdateUserCredentialsCommand(command); err != nil {
		return returnResult, err
	}
	// 口令只能走 POST /users/{id}/set-password（users:password:reset）。
	// PUT /credentials 保留给访问密钥轮换与 needUpdatePassword 管理。
	if command.Password != "" {
		return returnResult, apperrors.NewUnprocessableEntity("password updates must use POST /api/v1/users/{id}/set-password")
	}

	credManager := cred.GetCredManager(userdomain.PasswordAlgorithmArgon2ID)
	if credManager == nil {
		return returnResult, apperrors.NewInternal("argon2id credential manager unavailable")
	}

	now := s.Clock.Now().UTC()

	return returnResult, runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := loadUserForCredentials(ctx, tx, command.UserID, scopedOrganizationID)
		if err != nil {
			return err
		}

		// A write without a new password (access-secret rotation or the
		// need-update flag) keeps the stored algorithm. For a pre-existing legacy
		// row this re-persists the same value; no new legacy hash is created, and
		// the next password write below always selects argon2id.
		credentials := userdomain.UserCredentials{
			UserID:                    command.UserID,
			HasPassword:               user.Credentials.HasPassword,
			PasswordHash:              user.Credentials.PasswordHash,
			PasswordAlgo:              user.Credentials.PasswordAlgo,
			PasswordUpdatedAt:         user.Credentials.PasswordUpdatedAt,
			AccessKey:                 user.Credentials.AccessKey,
			AccessSecretHash:          user.Credentials.AccessSecretHash,
			AccessSecretRotatedAt:     user.Credentials.AccessSecretRotatedAt,
			NeedUpdatePassword:        user.Credentials.NeedUpdatePassword,
			AccessSecretHashPrev:      user.Credentials.AccessSecretHashPrev,
			AccessSecretPrevExpiresAt: user.Credentials.AccessSecretPrevExpiresAt,
		}
		if command.NeedUpdatePassword != nil {
			credentials.NeedUpdatePassword = *command.NeedUpdatePassword
		}

		if command.RotateAccessSecret {
			accessKey := generateClientID()
			accessSecret := generateClientSecret()
			accessSecretHash := credManager.GetHashedPassword(accessSecret, "")
			if accessSecretHash == "" {
				return apperrors.NewInternal("hash access secret")
			}

			// 宽限期：把当前的 access_secret_hash 滚动到 prev，
			// 并以本次轮换时间 + AccessSecretGracePeriod 设过期时间。
			// 语义是「自本次轮换起 N 分钟内旧 secret 仍可用」，与被轮换
			// secret 自身的激活时间无关，因此必须基于 now 而非旧 rotatedAt。
			if credentials.AccessSecretHash != "" {
				credentials.AccessSecretHashPrev = credentials.AccessSecretHash
				exp := now.Add(userdomain.AccessSecretGracePeriod)
				credentials.AccessSecretPrevExpiresAt = &exp
			}

			credentials.AccessKey = accessKey
			credentials.AccessSecretHash = accessSecretHash
			credentials.AccessSecretRotatedAt = &now
			returnResult.RotatedAccessKey = accessKey
			returnResult.RotatedAccessSecret = accessSecret
		}

		user.Credentials = credentials
		user.UpdatedAt = now
		user.Revision++

		if err := persistUserForCredentials(ctx, tx, user, scopedOrganizationID); err != nil {
			return err
		}

		if err := tx.Users().UpdateCredentials(ctx, &credentials); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			command.ActorID,
			"user.credentials.updated",
			"User",
			user.ID,
			"user",
			"user.credentials.updated",
			map[string]any{
				"targetUserId":        user.ID,
				"passwordChanged":     false,
				"accessSecretRotated": command.RotateAccessSecret,
			},
		); err != nil {
			return err
		}

		return nil
	})
}
