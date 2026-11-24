package sessions

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"time"

	"github.com/guliangiam/guliangiam/internal/app/uow"
	sessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type Service struct {
	Clock clock.Clock
	UOW   uow.AuthUoW
}

// 安全边界：PEP 中间件（APIAuthorizationMiddleware）已通过 Casbin 验证了权限。
// RLS 提升只在 service 内部生效（事务内 LOCAL set_config），事务结束自动清除。
// CleanupExpiredSessions permanently deletes sessions that have been expired
// for longer than the retention period. This prevents the sessions table from
// growing indefinitely.
//
// The retention parameter controls how long expired sessions are kept before
// deletion. A typical value is 7 days (168 hours), which allows recent
// expired sessions to remain queryable for audit purposes.
func (s *Service) CleanupExpiredSessions(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := s.Clock.Now().Add(-retention)
	deleted, err := s.UOW.Sessions().CleanupExpired(ctx, cutoff)
	if err != nil {
		slog.ErrorContext(ctx, "failed to cleanup expired sessions", "error", err, "cutoff", cutoff)
		return 0, err
	}
	if deleted > 0 {
		slog.InfoContext(ctx, "cleaned up expired sessions", "deleted", deleted, "cutoff", cutoff)
	}
	return deleted, nil
}

// RotateCurrentSession revokes the session identified by oldSessionID and issues
// a brand-new opaque session ID with the same user/app/organization context.
//
// Atomicity: runs inside its own transaction, so callers who already own a tx
// should just call Revoke() + Create() directly (which the login chain does).
// This helper is intended for standalone rotation scenarios — MFA setup,
// password reset, role grant — where the caller doesn't already hold a tx.
//
// Returns the new opaque session ID, or "" when oldSessionID is empty/not found.
func (s *Service) RotateCurrentSession(ctx context.Context, oldSessionID string) (string, error) {
	if oldSessionID == "" {
		return "", nil
	}

	now := s.Clock.Now().UTC()
	var newOpaque string

	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx uow.UnitOfWork) error {
		// Fetch old session to copy its context (user, app, device, expires).
		old, err := tx.Sessions().GetByID(ctx, oldSessionID)
		if err != nil {
			// Only a missing session means there is nothing to rotate. Any other
			// failure (database unavailable, RLS, …) must surface instead of
			// silently skipping rotation.
			if apperrors.FromError(err).Code == apperrors.CodeNotFound {
				return nil
			}
			slog.WarnContext(ctx, "failed to load session for rotation", "session_id", oldSessionID, "error", err)
			return err
		}

		// Revoke the old one (idempotent).
		if err := tx.Sessions().Revoke(ctx, oldSessionID, now); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			old.OrganizationID,
			old.UserID,
			"session.revoked",
			"Session",
			oldSessionID,
			"Session",
			"session.revoked",
			map[string]any{},
		); err != nil {
			return err
		}
		var err2 error
		newOpaque, err2 = randomOpaque(32)
		if err2 != nil {
			return err2
		}
		newSid, err2 := randomOpaque(24)
		if err2 != nil {
			return err2
		}

		newSession := &sessions.Session{
			ID:               newOpaque,
			Sid:              "sid-" + newSid,
			UserID:           old.UserID,
			Username:         old.Username,
			OrganizationID:   old.OrganizationID,
			OrganizationSlug: old.OrganizationSlug,
			ApplicationID:    old.ApplicationID,
			ApplicationSlug:  old.ApplicationSlug,
			SessionState:     "active",
			DeviceInfo:       old.DeviceInfo,
			IPAddress:        old.IPAddress,
			UserAgent:        old.UserAgent,
			ExpiresAt:        old.ExpiresAt,
			CreatedAt:        now,
			LastSeenAt:       now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if err := tx.Sessions().Create(ctx, newSession); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			newSession.OrganizationID,
			newSession.UserID,
			"session.created",
			"Session",
			newSession.ID,
			"Session",
			"session.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return newOpaque, nil
}

// randomOpaque generates a url-safe opaque token of the requested byte length.
func randomOpaque(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
