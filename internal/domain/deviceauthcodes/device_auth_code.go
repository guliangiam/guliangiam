package deviceauthcodes

import "time"

// DeviceAuthCode represents a pending device authorization stored in the database.
type DeviceAuthCode struct {
	ID              string     // UUIDv7 primary key
	DeviceCode      string     // The device_code used for polling
	UserCode        string     // The user_code displayed to the user
	ClientID        string     // OAuth client_id
	Scope           string     // Space-separated scopes
	ExpiresAt       time.Time  // When the device code expires
	PollIntervalSec int        // Polling interval in seconds
	Verified        bool       // Whether the user has verified the code
	VerifiedAt      *time.Time // When verification happened
	UserID          string     // User who verified
	Username        string     // Username of verifier
	OrganizationID  string     // Organization of verifier
	// LastPolledAt and SlowDownCount implement the RFC 8628 §3.5 throttle.
	// Without them the server cannot tell a well-behaved client from one
	// hammering the token endpoint, so "poll every interval seconds" was an
	// unenforceable instruction (设计报告 14.23.2).
	LastPolledAt  *time.Time
	SlowDownCount int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Revision      int64
}
