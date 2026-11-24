package lockoutattempts

import "time"

type attemptRow struct {
	ID              string
	UserID          string
	Attempts        int
	LockedUntil     *time.Time
	WindowStartedAt time.Time
	LockoutLevel    int
	Revision        int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
