package radiusconfigs

import "time"

type RadiusConfig struct {
	ID             string
	OrganizationID string
	Name           string
	Host           string
	Port           int
	SharedSecret   string
	AuthProtocol   string // PAP, CHAP
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
