package sessions

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func toSession(row sessionRow) (*domain.Session, error) {
	session := &domain.Session{
		ID:               row.ID,
		UserID:           row.UserID,
		Username:         row.Username,
		OrganizationID:   row.OrganizationID,
		OrganizationSlug: row.OrganizationSlug,
		ApplicationID:    row.ApplicationID,
		ApplicationSlug:  row.ApplicationSlug,
		SessionState:     domain.State(row.SessionState), Sid: derefString(row.Sid), IPAddress: row.IPAddress,
		UserAgent:           row.UserAgent,
		ExpiresAt:           row.ExpiresAt,
		RevokedAt:           row.RevokedAt,
		CreatedAt:           row.CreatedAt,
		LastSeenAt:          row.LastSeenAt,
		UpdatedAt:           row.UpdatedAt,
		Revision:            row.Revision,
		AuthenticationLevel: row.AuthenticationLevel,
		AssuranceLevel:      row.AssuranceLevel,
		IsPrivileged:        row.IsPrivileged,
		LifecycleID:         row.LifecycleID,
		TicketID:            row.TicketID,
		// Perf: defer DeviceInfo allocation — let UnmarshalJSON allocate the exact-size map;
		// only fall back to an empty map if unmarshal produces nil.
	}

	if err := shared.UnmarshalJSON(row.DeviceInfo, &session.DeviceInfo); err != nil {
		return nil, err
	}
	if session.DeviceInfo == nil {
		session.DeviceInfo = map[string]any{}
	}

	return session, nil
}
