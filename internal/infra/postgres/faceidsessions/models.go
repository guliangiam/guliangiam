package faceidsessions

import "time"

type sessionRow struct {
	ID           string
	SessionID    string
	Challenge    string
	UserID       string
	ExpiresAt    time.Time
	Verified     bool
	LivenessOK   bool
	AttemptCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
