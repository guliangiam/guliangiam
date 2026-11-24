package deviceauthcodes

import "time"

type deviceAuthCodeRow struct {
	ID              string
	DeviceCode      string
	UserCode        string
	ClientID        string
	Scope           string
	ExpiresAt       time.Time
	PollIntervalSec int
	Verified        bool
	VerifiedAt      *time.Time
	UserID          string
	Username        string
	OrganizationID  string
	LastPolledAt    *time.Time
	SlowDownCount   int
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Revision        int64
}
