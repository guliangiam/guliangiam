package users

import "time"

type UserIdentity struct {
	ID              string
	UserID          string
	ProviderID      string
	IdentityType    string
	Subject         string
	ProviderAccount string
	ProfileSnapshot map[string]any
	LinkedAt        time.Time
}
