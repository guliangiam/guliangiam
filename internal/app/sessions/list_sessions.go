package sessions

import (
	"context"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type ListSessionsQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	UserID         string
	ApplicationID  string
	State          string
	Keyword        string
	ActiveOnly     *bool
}

func (s *Service) ListSessions(ctx context.Context, query ListSessionsQuery) ([]SessionListItem, int64, error) {
	sessions, total, err := s.UOW.Sessions().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}

	items := make([]SessionListItem, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, toListItem(session))
	}

	return items, total, nil
}

// ListUserSessionsForOrganization returns one user's sessions only after the
// parent user is proven to belong to organizationID. sessions has no
// organization column; the parent user is the tenant anchor, and the child
// query also carries the organization predicate for defence in depth.
func (s *Service) ListUserSessionsForOrganization(ctx context.Context, organizationID, userID string, page, pageSize int) ([]SessionListItem, int64, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, 0, apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(userID) == "" {
		return nil, 0, apperrors.NewInvalidArgument("userId is required")
	}

	if _, err := s.UOW.Users().GetByIDForOrganization(ctx, userID, organizationID); err != nil {
		return nil, 0, err
	}

	return s.ListSessions(ctx, ListSessionsQuery{
		Page:           page,
		PageSize:       pageSize,
		OrganizationID: organizationID,
		UserID:         userID,
	})
}
