package users

import (
	"context"

	"github.com/guliangiam/guliangiam/internal/app/jml"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PatchUserCommand struct {
	OrganizationID      string
	ID                  string
	ExternalID          *string
	SignupApplicationID *string
	UserType            *string
	Status              *string
	IsAdmin             *bool
	IsForbidden         *bool
	IsDeleted           *bool
	IsOnline            *bool
	CreatedIp           *string
	Tag                 *string
	Score               *int
	Karma               *int
	Ranking             *int
	Language            *string
	Title               *string
	Balance             *float64
	BalanceCredit       *float64
	BalanceCurrency     *string
	IpWhitelist         *string
	Custom1             *string
	Custom2             *string
	Custom3             *string
	Custom4             *string
	Custom5             *string
	Custom6             *string
	Custom7             *string
	Custom8             *string
	Custom9             *string
	Custom10            *string
	Revision            int64
}

func (s *Service) PatchUser(ctx context.Context, command PatchUserCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if err := validatePatchUserCommand(command); err != nil {
		return err
	}
	// 规范 4.10：可编辑资源写路径必须携带乐观锁前提（本端点约定为 body
	// revision）；缺失不再默认最后写入获胜，直接 428。
	if command.Revision <= 0 {
		return apperrors.NewPreconditionRequired("user patch requires a revision precondition")
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation, so a cross-tenant request
		// leaves no write and no audit record behind.
		user, err := tx.Users().GetByIDForOrganization(ctx, command.ID, command.OrganizationID)
		if err != nil {
			return err
		}

		if user.Revision != command.Revision {
			// 412 携带当前 revision（规范 4.10：响应体指明当前版本），客户端
			// 重取后可直接重试；409 仅保留给唯一约束等非版本冲突。
			return apperrors.NewWithMeta(apperrors.CodePreconditionFailed,
				"user revision mismatch", map[string]any{"revision": user.Revision})
		}

		wasForbidden := user.IsForbidden

		if command.ExternalID != nil {
			user.ExternalID = *command.ExternalID
		}
		if command.SignupApplicationID != nil {
			user.SignupApplicationID = *command.SignupApplicationID
		}
		if command.UserType != nil {
			user.UserType = *command.UserType
		}
		if command.Status != nil {
			user.Status = *command.Status
		}
		if command.IsAdmin != nil {
			user.IsAdmin = *command.IsAdmin
		}
		if command.IsForbidden != nil {
			user.IsForbidden = *command.IsForbidden
		}
		if command.IsDeleted != nil {
			user.IsDeleted = *command.IsDeleted
		}
		if command.IsOnline != nil {
			user.IsOnline = *command.IsOnline
		}
		if command.CreatedIp != nil {
			user.CreatedIp = *command.CreatedIp
		}
		if command.Tag != nil {
			user.Tag = *command.Tag
		}
		if command.Score != nil {
			user.Score = *command.Score
		}
		profileChanged := false
		if command.Karma != nil || command.Ranking != nil || command.Language != nil || command.Title != nil {
			if user.Profile.Properties == nil {
				user.Profile.Properties = make(map[string]any)
			}
			if command.Karma != nil {
				user.Profile.Properties["karma"] = *command.Karma
			}
			if command.Ranking != nil {
				user.Profile.Properties["ranking"] = *command.Ranking
			}
			if command.Language != nil {
				user.Profile.Properties[profileKeyLanguage] = *command.Language
			}
			if command.Title != nil {
				user.Profile.Properties[profileKeyTitle] = *command.Title
			}
			profileChanged = true
		}
		if command.Balance != nil {
			user.Balance = *command.Balance
		}
		if command.BalanceCredit != nil {
			user.BalanceCredit = *command.BalanceCredit
		}
		if command.BalanceCurrency != nil {
			user.BalanceCurrency = *command.BalanceCurrency
		}
		if command.IpWhitelist != nil {
			user.IpWhitelist = *command.IpWhitelist
		}
		if command.Custom1 != nil {
			user.Custom1 = *command.Custom1
		}
		if command.Custom2 != nil {
			user.Custom2 = *command.Custom2
		}
		if command.Custom3 != nil {
			user.Custom3 = *command.Custom3
		}
		if command.Custom4 != nil {
			user.Custom4 = *command.Custom4
		}
		if command.Custom5 != nil {
			user.Custom5 = *command.Custom5
		}
		if command.Custom6 != nil {
			user.Custom6 = *command.Custom6
		}
		if command.Custom7 != nil {
			user.Custom7 = *command.Custom7
		}
		if command.Custom8 != nil {
			user.Custom8 = *command.Custom8
		}
		if command.Custom9 != nil {
			user.Custom9 = *command.Custom9
		}
		if command.Custom10 != nil {
			user.Custom10 = *command.Custom10
		}

		user.UpdatedAt = s.Clock.Now()
		user.Revision++

		if err := tx.Users().UpdateForOrganization(ctx, user, command.OrganizationID); err != nil {
			return err
		}
		if profileChanged {
			if err := tx.Users().UpdateProfile(ctx, &user.Profile); err != nil {
				return err
			}
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			"user.updated",
			"User",
			user.ID,
			"user",
			"user.updated",
			map[string]any{},
		); err != nil {
			return err
		}

		// Leaver（ADR-030）：停用转变同事务吊销会话/令牌并投递下游停用事件。
		if s.JML != nil && !wasForbidden && user.IsForbidden {
			if err := s.JML.OnUserDeactivated(ctx, tx, user.ID, user.OrganizationID, jml.SafeUserPayload(user)); err != nil {
				return err
			}
		}
		return nil
	})
}
