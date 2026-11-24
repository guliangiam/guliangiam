package radiusconfigs

import "time"

type radiusConfigRow struct {
	ID             string
	OrganizationID string
	Name           string
	Host           string
	Port           int
	SharedSecret   string
	AuthProtocol   string
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
