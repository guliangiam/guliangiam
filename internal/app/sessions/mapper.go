package sessions

import domain "github.com/guliangiam/guliangiam/internal/domain/sessions"

func toListItem(session *domain.Session) SessionListItem {
	return SessionListItem{
		ID:               session.ID,
		UserID:           session.UserID,
		Username:         session.Username,
		OrganizationID:   session.OrganizationID,
		OrganizationSlug: session.OrganizationSlug,
		ApplicationID:    session.ApplicationID,
		ApplicationSlug:  session.ApplicationSlug,
		SessionState:     string(session.SessionState),
		IPAddress:        session.IPAddress,
		UserAgent:        session.UserAgent,
		ExpiresAt:        session.ExpiresAt,
		RevokedAt:        session.RevokedAt,
		CreatedAt:        session.CreatedAt,
		LastSeenAt:       session.LastSeenAt,
		UpdatedAt:        session.UpdatedAt,
	}
}

func toDetail(session *domain.Session) SessionDetail {
	deviceInfo := session.DeviceInfo
	if deviceInfo != nil {
		// The provider access token is session-scoped state for logout fan-out,
		// not API data. Drop it before the map reaches any client.
		deviceInfo = make(map[string]any, len(session.DeviceInfo))
		for key, value := range session.DeviceInfo {
			if key == domain.DeviceInfoProviderAccessToken {
				continue
			}
			deviceInfo[key] = value
		}
	}
	if deviceInfo == nil {
		deviceInfo = map[string]any{}
	}

	return SessionDetail{
		SessionListItem: toListItem(session),
		DeviceInfo:      deviceInfo,
		Revision:        session.Revision,
	}
}
