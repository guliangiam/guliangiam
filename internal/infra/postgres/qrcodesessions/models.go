package qrcodesessions

import "time"

type sessionRow struct {
	ID             string
	CodeID         string
	Code           string
	OrganizationID string
	ApplicationID  string
	ExpiresAt      time.Time
	Status         string
	UserID         string
	Username       string
	IPAddress      string
	UserAgent      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
