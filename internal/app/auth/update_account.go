package auth

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// UpdateAccountCommand 是会话主体对**自己**账号的自助更新（编辑资料 + 绑定联系方式）。
//
// 与 GET /api/v1/auth/account 同路径（PATCH）：任何已登录用户都能改自己的资料，包括
// 扫码登录自动注册出来的 guest 账号——不需要管理面 users:update 权限，这正是访客账号
// 能被「认领」的路径。
type UpdateAccountCommand struct {
	OrganizationSlug string
	Username         string
	AccessToken      string

	// DisplayName/AvatarURL 为可选的资料编辑（指针语义：nil 表示不改）。
	DisplayName *string
	AvatarURL   *string
	// Email/Phone 为要绑定的新联系方式；绑定必须带发给**该联系方式**的验证码，
	// 校验通过才写入，避免把别人的邮箱/手机号绑到自己账号上。
	Email            *string
	Phone            *string
	VerificationCode string
}

// UpdateAccount 更新当前会话主体的资料，并按需绑定（已验证的）联系方式。
func (s *Service) UpdateAccount(ctx context.Context, command UpdateAccountCommand) (*AccountContext, error) {
	if strings.TrimSpace(command.OrganizationSlug) == "" || strings.TrimSpace(command.Username) == "" {
		return nil, apperrors.NewUnauthorized("authentication required")
	}
	if command.Email == nil && command.Phone == nil && command.DisplayName == nil && command.AvatarURL == nil {
		return nil, apperrors.NewInvalidArgument("nothing to update")
	}

	// 联系方式绑定一次只能处理一个目标，因为 verificationCode 只能属于
	// 单个 recipient（要么 email 要么 phone）。自助端 UI 也是二选一，同时
	// 传两者的调用方必然遗漏其中之一的验证码 → fail-closed 拒绝，而不是
	// 让第二次 consume 在第一次消费后静默失败。
	if command.Email != nil && command.Phone != nil {
		return nil, apperrors.NewWithMeta(
			apperrors.CodeInvalidArgument,
			"cannot change email and phone in the same request",
			map[string]any{
				"reason":     "verificationCode is bound to a single recipient",
				"suggestion": "issue two separate requests, one for email and one for phone",
			},
		)
	}

	var result *AccountContext
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		organization, err := tx.Organizations().GetBySlug(ctx, command.OrganizationSlug)
		if err != nil {
			return err
		}

		user, err := tx.Users().GetByUsername(ctx, organization.ID, command.Username)
		if err != nil {
			return err
		}

		profile := user.Profile

		if command.Email != nil {
			email := strings.TrimSpace(*command.Email)
			if email == "" {
				return apperrors.NewInvalidArgument("email is required")
			}
			if err := s.consumeContactVerification(ctx, tx, organization.ID, email, "email", command.VerificationCode); err != nil {
				return err
			}
			profile.Email = email
		}

		if command.Phone != nil {
			phone := strings.TrimSpace(*command.Phone)
			if phone == "" {
				return apperrors.NewInvalidArgument("phone is required")
			}
			if err := s.consumeContactVerification(ctx, tx, organization.ID, phone, "phone", command.VerificationCode); err != nil {
				return err
			}
			profile.Phone = phone
		}

		if command.DisplayName != nil {
			if displayName := strings.TrimSpace(*command.DisplayName); displayName != "" {
				profile.DisplayName = displayName
			}
		}
		if command.AvatarURL != nil {
			if avatarURL := strings.TrimSpace(*command.AvatarURL); avatarURL != "" {
				profile.AvatarURL = avatarURL
			}
		}
		profile.UserID = user.ID

		if err := tx.Users().UpdateProfile(ctx, &profile); err != nil {
			return err
		}
		user.Profile = profile

		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"",
			user.OrganizationID,
			user.ID,
			"user.updated",
			"user",
			user.ID,
			"user",
			"user.updated",
			map[string]any{"selfService": true},
		); err != nil {
			return err
		}

		account := toAccountContext(organization, user, command.AccessToken)
		result = &account
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// consumeContactVerification 校验并消费「发给待绑定联系方式」的验证码（一次性）。
// organizationID 绑定码的签发租户：他组织码与本组织无效码同形拒绝（BE-A-20）。
func (s *Service) consumeContactVerification(ctx context.Context, tx appuow.UnitOfWork, organizationID, recipient, verificationType, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return apperrors.NewInvalidArgument("verificationCode is required to bind a contact")
	}

	verification, err := matchVerificationCode(ctx, tx.Verifications(), organizationID, recipient, verificationType, code)
	if err != nil {
		return verificationMatchError(err, "verification code is invalid or expired")
	}

	return tx.Verifications().UpdateStatus(ctx, verification.ID, "consumed")
}
