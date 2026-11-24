package sessions

import "time"

type sessionRow struct {
	ID                  string
	UserID              string
	Username            string
	OrganizationID      string
	OrganizationSlug    string
	ApplicationID       string
	ApplicationSlug     string
	SessionState        string
	Sid                 *string
	DeviceInfo          []byte
	IPAddress           string
	UserAgent           string
	ExpiresAt           time.Time
	RevokedAt           *time.Time
	CreatedAt           time.Time
	LastSeenAt          time.Time
	UpdatedAt           time.Time
	Revision            int64
	AuthenticationLevel string
	AssuranceLevel      string
	IsPrivileged        bool
	LifecycleID         string
	TicketID            string
}
