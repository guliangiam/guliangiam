package radiusconfigs

import "time"

// radiusConfigRow 是 radiusconfigs 表的行模型（无 JSON 字段，直接映射）。
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
