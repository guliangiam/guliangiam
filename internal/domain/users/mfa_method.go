package users

import "time"

type UserMfaMethod struct {
	ID          string
	UserID      string
	MethodType  string
	Enabled     bool
	Verified    bool
	DisplayName string
	LastUsedAt  *time.Time
}
