package sessions

import "time"

type SessionListItem struct {
	ID               string     `json:"id"`
	UserID           string     `json:"userId"`
	Username         string     `json:"username"`
	OrganizationID   string     `json:"organizationId"`
	OrganizationSlug string     `json:"organizationSlug"`
	ApplicationID    string     `json:"applicationId"`
	ApplicationSlug  string     `json:"applicationSlug"`
	SessionState     string     `json:"sessionState"`
	IPAddress        string     `json:"ipAddress"`
	UserAgent        string     `json:"userAgent"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	LastSeenAt       time.Time  `json:"lastSeenAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type SessionDetail struct {
	SessionListItem
	DeviceInfo map[string]any `json:"deviceInfo"`
	Revision   int64          `json:"revision"`
}
