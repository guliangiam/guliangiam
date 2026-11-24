package sessions

import (
	"context"
	"strings"

	"github.com/guliangiam/guliangiam/internal/app/sessionpolicy"
	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) GetCurrentSession(ctx context.Context, id string) (*SessionDetail, error) {
	// sessions 表 qual 拦截 SELECT（organization_id != current_org）。
	session, err := s.requireActiveSession(ctx, id)
	if err != nil {
		return nil, err
	}

	detail := toDetail(session)
	return &detail, nil
}

func (s *Service) requireActiveSession(ctx context.Context, id string) (*domain.Session, error) {
	if strings.TrimSpace(id) == "" {
		return nil, apperrors.NewInvalidArgument("session id is required")
	}
	session, err := s.UOW.Sessions().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now().UTC()
	idleTimeout, err := sessionpolicy.ResolveIdleTimeout(ctx, s.UOW.Organizations(), session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if session.RevokedAt != nil {
		return nil, apperrors.NewUnauthorized("session is revoked")
	}
	if !session.ExpiresAt.After(now) || sessionpolicy.IsIdleExpired(now, session.LastSeenAt, idleTimeout) {
		return nil, apperrors.NewUnauthorized("session is expired")
	}
	if sessionpolicy.ShouldTouchActivity(now, session.LastSeenAt, idleTimeout) {
		if err := s.UOW.Sessions().Touch(ctx, session.ID, now); err != nil {
			return nil, err
		}
		session.LastSeenAt = now
		session.UpdatedAt = now
	}

	return session, nil
}
