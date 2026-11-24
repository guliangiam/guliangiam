package users

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/guliangiam/guliangiam/cred"
	"github.com/guliangiam/guliangiam/internal/app/jml"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type CreateUserProfileCommand struct {
	DisplayName string
	FirstName   string
	LastName    string
	AvatarURL   string
	Email       string
	Phone       string
	CountryCode string
	Region      string
	Location    string
	Gender      string
	Birthday    string
	Bio         string
	Homepage    string
	Education   string
	Language    string
	RealName    string
	IDCardType  string
	IDCard      string
	Affiliation string
	Title       string
}

type CreateUserWithProfileCommand struct {
	CreateUserCommand
	Profile               CreateUserProfileCommand
	InitialPassword       string
	RequirePasswordChange bool
	IsForbidden           bool
	IsDeleted             bool
	IpWhitelist           string
	Tag                   string
	Score                 int
	Karma                 int
	Ranking               int
	Balance               float64
	BalanceCredit         float64
	BalanceCurrency       string
	SignupApplicationID   string
	RoleBindingIDs        []string
	GroupIDs              []string
}

func (s *Service) CreateUser(ctx context.Context, command CreateUserWithProfileCommand) (*userdomain.User, error) {
	// Transport converges command.OrganizationID through TrustedOrganizationID
	// before calling this use case; direct callers must still pass a tenant that
	// the caller's scope authorizes.
	if err := validateCreateUserCommand(command); err != nil {
		return nil, err
	}

	user, err := s.NewUser(command.CreateUserCommand)
	if err != nil {
		return nil, err
	}

	user.IsForbidden = command.IsForbidden
	user.IsDeleted = command.IsDeleted
	user.IpWhitelist = command.IpWhitelist
	user.Tag = command.Tag
	user.Score = command.Score
	user.Balance = command.Balance
	user.BalanceCredit = command.BalanceCredit
	if command.BalanceCurrency != "" {
		user.BalanceCurrency = command.BalanceCurrency
	}
	if command.SignupApplicationID != "" {
		user.SignupApplicationID = command.SignupApplicationID
	}

	profile := userdomain.UserProfile{
		UserID:      user.ID,
		DisplayName: command.Profile.DisplayName,
		FirstName:   command.Profile.FirstName,
		LastName:    command.Profile.LastName,
		AvatarURL:   command.Profile.AvatarURL,
		Email:       command.Profile.Email,
		Phone:       command.Profile.Phone,
		CountryCode: command.Profile.CountryCode,
		Region:      command.Profile.Region,
		Location:    command.Profile.Location,
		Gender:      command.Profile.Gender,
		Birthday:    command.Profile.Birthday,
		Bio:         command.Profile.Bio,
		RealName:    command.Profile.RealName,
		IDCardType:  command.Profile.IDCardType,
		IDCard:      command.Profile.IDCard,
		Affiliation: command.Profile.Affiliation,
	}

	profileProperties := map[string]any{
		"karma":   command.Karma,
		"ranking": command.Ranking,
	}
	for key, value := range map[string]string{
		profileKeyAffiliation: profile.Affiliation,
		profileKeyEducation:   command.Profile.Education,
		profileKeyHomepage:    command.Profile.Homepage,
		profileKeyLanguage:    command.Profile.Language,
		profileKeyTitle:       command.Profile.Title,
	} {
		if value != "" {
			profileProperties[key] = value
		}
	}
	if len(profileProperties) > 0 {
		profile.Properties = profileProperties
	}

	// repository.Create 内部会调 createProfileTx(ctx, tx, &user.Profile)，
	// 必须把拼好的 profile 赋给 user，否则 user.Profile.UserID 是空字符串。
	user.Profile = profile

	roleBindings := make([]userdomain.UserRoleBinding, 0, len(command.RoleBindingIDs))
	for _, roleID := range command.RoleBindingIDs {
		roleBindings = append(roleBindings, userdomain.UserRoleBinding{UserID: user.ID, RoleID: roleID})
	}

	groupMemberships := make([]userdomain.UserGroupMembership, 0, len(command.GroupIDs))
	for _, groupID := range command.GroupIDs {
		groupMemberships = append(groupMemberships, userdomain.UserGroupMembership{UserID: user.ID, GroupID: groupID})
	}

	// BE-A-08: 组织解析、口令策略校验与 argon2 哈希全部在事务外完成，
	// 避免 CPU 密集的哈希拉长事务、占用连接与锁。
	// command.OrganizationID 可能是 slug 或 uuid；resolve 成 uuid 才能参与 DB 查询。
	var org *orgdomain.Organization
	if _, parseErr := uuid.Parse(user.OrganizationID); parseErr != nil {
		resolved, err := s.UOW.Organizations().GetBySlug(ctx, user.OrganizationID)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "resolve organization %q", user.OrganizationID)
		}
		org = resolved
		user.OrganizationID = resolved.ID
	} else if command.InitialPassword != "" {
		resolved, err := s.UOW.Organizations().GetByID(ctx, user.OrganizationID)
		if err != nil {
			return nil, err
		}
		org = resolved
	}
	slog.InfoContext(ctx, "createUser: org resolved", "organization_id", user.OrganizationID, "username", user.Username)

	// 管理员建号时的初始口令同样按组织策略校验。
	var passwordHash string
	if command.InitialPassword != "" {
		if err := validatePasswordComplexity(command.InitialPassword, command.Username, orgdomain.EffectivePasswordPolicy(org)); err != nil {
			return nil, err
		}
		credManager := cred.GetCredManager(userdomain.PasswordAlgorithmArgon2ID)
		if credManager == nil {
			return nil, apperrors.NewInternal("argon2id credential manager unavailable")
		}
		passwordHash = credManager.GetHashedPassword(command.InitialPassword, "")
		if passwordHash == "" {
			return nil, apperrors.NewInternal("hash user password")
		}
	}

	if err := runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Check for active user with the same username in the organization.
		_, err := tx.Users().GetByUsername(ctx, user.OrganizationID, user.Username)
		if err == nil {
			return apperrors.NewAlreadyExists("username already exists in organization")
		}
		var appErr *apperrors.Error
		if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
			return err
		}

		// Check for soft-deleted user with the same username.
		// The DB unique constraint on (organization_id, username) does not
		// include deleted_at, so a soft-deleted record would cause a
		// constraint violation on INSERT.
		hasDeleted, err := tx.Users().HasDeletedWithUsername(ctx, user.OrganizationID, user.Username)
		if err != nil {
			return err
		}
		if hasDeleted {
			return apperrors.NewAlreadyExists("username already exists in organization (soft-deleted); permanently delete the existing user or use a different username")
		}

		// Role/group ids are tenant resources: a foreign or missing id must
		// reject the whole creation before any write or audit (mirrors
		// ReplaceUserRoleBindings / ReplaceUserGroupMemberships).
		for _, roleID := range command.RoleBindingIDs {
			if _, err := tx.Roles().GetByIDForOrganization(ctx, roleID, user.OrganizationID); err != nil {
				return err
			}
		}
		for _, groupID := range command.GroupIDs {
			if _, err := tx.Groups().GetByIDForOrganization(ctx, groupID, user.OrganizationID); err != nil {
				return err
			}
		}

		// 先 Insert 主实体 — records FK 必须引用已存在的行
		if err := tx.Users().Create(ctx, user); err != nil {
			return err
		}

		// 再写 audit + outbox（此时 user 行已在事务内，FK 能过）
		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			command.ActorID,
			"user.created",
			"User",
			user.ID,
			"user",
			"user.created",
			map[string]any{},
		); err != nil {
			return err
		}

		if command.InitialPassword != "" {
			now := s.Clock.Now()
			credentials := &userdomain.UserCredentials{
				UserID:             user.ID,
				PasswordHash:       passwordHash,
				PasswordAlgo:       userdomain.PasswordAlgorithmArgon2ID,
				PasswordUpdatedAt:  &now,
				HasPassword:        true,
				NeedUpdatePassword: command.RequirePasswordChange,
			}
			if err := tx.Users().UpdateCredentials(ctx, credentials); err != nil {
				return err
			}
		}
		if s.JML != nil {
			// Joiner（ADR-030）：初始角色由 JML 逐条创建并做 SoD 预检，
			// 冲突返回错误 → 整个建号事务回滚。
			if err := s.JML.OnUserCreated(ctx, tx, user.ID, user.OrganizationID, jml.SafeUserPayload(user), command.RoleBindingIDs); err != nil {
				return err
			}
		} else if len(roleBindings) > 0 {
			if err := tx.Users().ReplaceRoleBindings(ctx, user.ID, roleBindings); err != nil {
				return err
			}
		}
		if len(groupMemberships) > 0 {
			if err := tx.Users().ReplaceGroupMemberships(ctx, user.ID, groupMemberships); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// Project the persisted role bindings into Casbin before the user can sign
	// in; otherwise role-based authorization never sees them.
	// BE-A-07: 用户已提交成功，resync 失败不应让调用方误判创建失败并重试；
	// 降级为告警，后续登录/角色变更路径会再次投影。
	if err := s.resyncUserRoles(ctx, user.ID); err != nil {
		slog.WarnContext(ctx, "createUser: post-commit role policy resync failed",
			"user_id", user.ID, "organization_id", user.OrganizationID, "error", err)
	}

	return user, nil
}
