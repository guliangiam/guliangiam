package sessions

import (
	"context"
	"strings"

	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) GetSession(ctx context.Context, id string) (*SessionDetail, error) {
	return s.getSession(ctx, id, "")
}

// GetSessionForOrganization is the management by-ID read. Ownership is resolved
// through users.organization_id (sessions has no organization_id column), so a
// foreign session is indistinguishable from a missing one.
func (s *Service) GetSessionForOrganization(ctx context.Context, id, organizationID string) (*SessionDetail, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	return s.getSession(ctx, id, organizationID)
}

func (s *Service) getSession(ctx context.Context, id, organizationID string) (*SessionDetail, error) {
	var (
		session *domainsessions.Session
		err     error
	)
	if organizationID == "" {
		// 系统/内部路径：PG 由 RLS 兜底，MySQL 无 RLS。
		session, err = s.UOW.Sessions().GetByID(ctx, id)
	} else {
		session, err = s.UOW.Sessions().GetByIDForOrganization(ctx, id, organizationID)
	}
	if err != nil {
		return nil, err
	}

	detail := toDetail(session)
	return &detail, nil
}
