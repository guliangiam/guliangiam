package faceidsessions

import "time"

// Session represents a Face ID verification session stored in the database.
type Session struct {
	ID           string    // UUIDv7 primary key
	SessionID    string    // Unique session identifier
	Challenge    string    // Random challenge for liveness detection
	UserID       string    // User being verified
	ExpiresAt    time.Time // When the session expires
	Verified     bool      // Whether face verification succeeded
	LivenessOK   bool      // Whether liveness detection passed
	AttemptCount int       // Failed verification attempts (>= faceIDMaxAttempts triggers session deletion)
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Revision     int64
}
