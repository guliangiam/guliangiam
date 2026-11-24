package sessions

import (
	"context"

	"log/slog"
	"strings"

	uow "github.com/guliangiam/guliangiam/internal/app/uow"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokensdomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// ForceLogoutResult is returned by the administrative force-logout endpoints.
// AC-RISK-002 requires the "revocation list" to be accurate — callers can
// audit how many sessions, refresh tokens, and access tokens died together.
type ForceLogoutResult struct {
	SessionCount int64
	RefreshCount int64
	AccessCount  int64
}

// ForceLogoutUser revokes all active sessions + refresh tokens + access tokens
// for the given user. This is the administrative "force logout" / emergency
// revocation operation (AC-RISK-005, AC-AUTH-009). It does not exclude any
// session, including the caller's own.
//
// Returns session/refresh/access counts for audit. Runs inside its own
// transaction so callers can safely invoke it from a read-only path or a
// standalone handler.
func (s *Service) ForceLogoutUser(ctx context.Context, userID string) (*ForceLogoutResult, error) {
	return s.forceLogoutUser(ctx, "", userID)
}

// ForceLogoutUserForOrganization is the management-route variant: {id} comes
// from the URL, so the target user is resolved through the trusted tenant
// before any session or token is revoked.
func (s *Service) ForceLogoutUserForOrganization(ctx context.Context, organizationID, userID string) (*ForceLogoutResult, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(userID) == "" {
		return nil, apperrors.NewInvalidArgument("userId is required")
	}
	return s.forceLogoutUser(ctx, organizationID, userID)
}

func (s *Service) forceLogoutUser(ctx context.Context, organizationID, userID string) (*ForceLogoutResult, error) {
	// 用户可能不在本 org → 被 RLS 拦截。
	revokedAt := s.Clock.Now().UTC()

	result := &ForceLogoutResult{}
	var user *userdomain.User
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx uow.UnitOfWork) error {
		var err error

		if organizationID != "" {
			user, err = tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
			if err != nil {
				return err
			}
		}

		// Collect affected organizations before the sessions disappear so the
		// CAEP fan-out can target each tenant (users can span organizations).
		// BE-A-12: 翻页收集全部活跃会话；此前只取第一页 100 条，超出部分
		// 的组织不会收到 SSF 吊销事件。
		active := true
		affected, err := pagination.CollectAll(func(page, pageSize int) ([]*sessionsdomain.Session, int64, error) {
			query := sessionsdomain.ListQuery{UserID: userID, ActiveOnly: &active, Page: page, PageSize: pageSize}
			if organizationID != "" {
				query.OrganizationID = organizationID
			}
			return tx.Sessions().List(ctx, query)
		})
		if err != nil {
			return err
		}
		affectedOrgs := make(map[string]struct{}, len(affected))
		for _, session := range affected {
			if session.OrganizationID != "" {
				affectedOrgs[session.OrganizationID] = struct{}{}
			}
		}

		// 1) Every active session for the user — every logged-in device dies.
		result.SessionCount, err = tx.Sessions().RevokeByUser(ctx, userID, revokedAt)
		if err != nil {
			return err
		}

		// 2) All refresh tokens + their paired access tokens (WITH CTE).
		result.RefreshCount, err = tx.Tokens().RevokeRefreshByUser(ctx, userID, revokedAt)
		if err != nil {
			return err
		}

		// 3) Remaining access tokens for this user (not paired with a refresh).
		result.AccessCount, err = tx.Tokens().RevokeByQuery(ctx, tokensdomain.RevokeQuery{UserID: userID}, revokedAt)
		if err != nil {
			return err
		}

		for orgID := range affectedOrgs {
			if err := s.enqueueSSFRevoked(ctx, tx, orgID, userID, "", "user_force_logout"); err != nil {
				return err
			}
		}

		if user != nil {
			return s.embedAuditEvent(
				tx,
				ctx,
				user.OrganizationID,
				"",
				"user.sessions.force_logged_out",
				"User",
				userID,
				"user",
				"user.sessions.force_logged_out",
				map[string]any{
					"sessionsRevoked": result.SessionCount,
					"refreshRevoked":  result.RefreshCount,
					"accessRevoked":   result.AccessCount,
				},
			)
		}
		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, "force-logout user failed", "user_id", userID, "error", err)
		return nil, err
	}

	slog.InfoContext(ctx, "force-logout user completed",
		"user_id", userID,
		"sessions_revoked", result.SessionCount,
		"refresh_revoked", result.RefreshCount,
		"access_revoked", result.AccessCount)
	return result, nil
}

// ForceLogoutOrganization revokes every active session + token in the given
// organization. Used for org-wide emergency lockdown (AC-RISK-005).
//
// Same cascade as ForceLogoutUser but scoped to OrganizationID — refresh
// tokens don't carry an org column, so RevokeByQuery handles both refresh +
// access rows in one call (refreshCount stays 0 here by design — they're
// folded into accessCount).
func (s *Service) ForceLogoutOrganization(ctx context.Context, organizationID string) (*ForceLogoutResult, error) {
	revokedAt := s.Clock.Now().UTC()

	result := &ForceLogoutResult{}
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx uow.UnitOfWork) error {
		var err error
		result.SessionCount, err = tx.Sessions().RevokeByOrganization(ctx, organizationID, revokedAt)
		if err != nil {
			return err
		}
		result.AccessCount, err = tx.Tokens().RevokeByQuery(ctx, tokensdomain.RevokeQuery{OrganizationID: organizationID}, revokedAt)
		if err != nil {
			return err
		}
		if err := s.enqueueSSFRevoked(ctx, tx, organizationID, "", "", "organization_force_logout"); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, "force-logout organization failed", "organization_id", organizationID, "error", err)
		return nil, err
	}

	slog.InfoContext(ctx, "force-logout organization completed",
		"organization_id", organizationID,
		"sessions_revoked", result.SessionCount,
		"refresh_revoked", result.RefreshCount,
		"access_revoked", result.AccessCount)
	return result, nil
}
