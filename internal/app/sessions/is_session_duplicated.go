package sessions

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
)

// IsSessionDuplicated checks whether a user has multiple active sessions,
// indicating a potential concurrent login or session duplication scenario.
func (s *Service) IsSessionDuplicated(ctx context.Context, userID, currentSessionID string) (bool, error) {
	activeOnly := true
	sessions, _, err := s.UOW.Sessions().List(ctx, domain.ListQuery{
		Page: 1, PageSize: 10, UserID: userID, ActiveOnly: &activeOnly,
	})
	if err != nil {
		return false, err
	}
	// If there are active sessions beyond the current one, it's a duplication
	activeCount := 0
	for _, session := range sessions {
		if session.ID != currentSessionID && session.RevokedAt == nil {
			activeCount++
		}
	}
	return activeCount > 0, nil
}
