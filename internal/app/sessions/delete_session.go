package sessions

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) DeleteSession(ctx context.Context, id string) error {
	return s.deleteSession(ctx, id, "")
}

// DeleteSessionForOrganization deletes only a session owned by organizationID.
// The scoped read happens inside the transaction before the delete, and the
// audit event records the entity's real organization.
func (s *Service) DeleteSessionForOrganization(ctx context.Context, id, organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return s.deleteSession(ctx, id, organizationID)
}

func (s *Service) deleteSession(ctx context.Context, id, organizationID string) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var (
			session *domainsessions.Session
			err     error
		)
		if organizationID == "" {
			// 平台管理员可能删除非本 org 的 session → 被 RLS 拦截。
			session, err = tx.Sessions().GetByID(ctx, id)
		} else {
			session, err = tx.Sessions().GetByIDForOrganization(ctx, id, organizationID)
		}
		if err != nil {
			return err
		}

		if organizationID == "" {
			err = tx.Sessions().Delete(ctx, id)
		} else {
			err = tx.Sessions().DeleteForOrganization(ctx, id, organizationID)
		}
		if err != nil {
			return err
		}

		return s.embedAuditEvent(
			tx,
			ctx,
			session.OrganizationID,
			session.UserID,
			"session.deleted",
			"session",
			id,
			"session",
			"session.deleted",
			map[string]any{keySessionID: id, "userId": session.UserID, keyOrgID: session.OrganizationID},
		)
	})
}
