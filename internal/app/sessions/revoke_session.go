package sessions

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) RevokeSession(ctx context.Context, id string) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.revokeSessionInTx(ctx, tx, id, "", "api")
	})
}

// RevokeSessionForOrganization revokes only a session owned by organizationID.
// The scoped read and the scoped revoke share one transaction, so a foreign
// session is rejected before any side effect (SSF event or audit record).
func (s *Service) RevokeSessionForOrganization(ctx context.Context, id, organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.revokeSessionInTx(ctx, tx, id, organizationID, "api")
	})
}

// RevokeUserSessionForOrganization revokes one session only when both the
// parent user and the session belong to organizationID. {id} comes from the
// URL, so the session's user binding is proven before any write.
func (s *Service) RevokeUserSessionForOrganization(ctx context.Context, organizationID, userID, id string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(userID) == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if strings.TrimSpace(id) == "" {
		return apperrors.NewInvalidArgument("sessionId is required")
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if _, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID); err != nil {
			return err
		}
		sess, err := tx.Sessions().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		// 属主不符按「找不到」处理，不泄露会话归属。
		if sess.UserID != userID {
			return apperrors.NewNotFound("session not found")
		}
		return s.revokeSessionEntityInTx(ctx, tx, sess, organizationID, id, "api")
	})
}

// RevokeSessionInTx revokes a session using the caller's transaction so the
// caller's own writes and the revocation commit or roll back together.
//
// Cross-domain callers (risk governance cascade) must use this instead of
// RevokeSession: opening a second transaction inside an active one works on
// PostgreSQL only by accident (it grabs another connection) and is rejected
// outright by the MySQL unit of work, which silently leaves the session alive.
func (s *Service) RevokeSessionInTx(ctx context.Context, tx appuow.UnitOfWork, id, trigger string) error {
	return s.revokeSessionInTx(ctx, tx, id, "", trigger)
}

func (s *Service) revokeSessionInTx(ctx context.Context, tx appuow.UnitOfWork, id, organizationID, trigger string) error {
	var (
		sess *domainsessions.Session
		err  error
	)
	if organizationID == "" {
		// 平台管理员可能 revoke 非本 org 的 session → 被 RLS 拦截。
		sess, err = tx.Sessions().GetByID(ctx, id)
	} else {
		sess, err = tx.Sessions().GetByIDForOrganization(ctx, id, organizationID)
	}
	if err != nil {
		return err
	}

	return s.revokeSessionEntityInTx(ctx, tx, sess, organizationID, id, trigger)
}

// revokeSessionEntityInTx applies the already-owned session row: scoped revoke,
// SSF fan-out and audit all use the entity's real organization.
func (s *Service) revokeSessionEntityInTx(ctx context.Context, tx appuow.UnitOfWork, sess *domainsessions.Session, organizationID, id, trigger string) error {
	var err error
	if organizationID == "" {
		err = tx.Sessions().Revoke(ctx, id, s.Clock.Now().UTC())
	} else {
		err = tx.Sessions().RevokeForOrganization(ctx, id, organizationID, s.Clock.Now().UTC())
	}
	if err != nil {
		return err
	}
	if err := s.enqueueSSFSessionRevoked(ctx, tx, sess.OrganizationID, sess.UserID, id); err != nil {
		return err
	}
	return s.embedAuditEvent(
		tx,
		ctx,
		sess.OrganizationID,
		sess.UserID,
		"session.revoked",
		"session",
		id,
		"session",
		"session.revoked",
		map[string]any{keySessionID: id, keyOrgID: sess.OrganizationID, "trigger": trigger},
	)
}
