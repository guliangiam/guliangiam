package agents

import "time"

// agentRow 是 MySQL agents 表的扫描目标。15 列。
type agentRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Type           string
	Host           string
	Port           int
	Url            string
	Token          string
	ApplicationID  string
	IsEnabled      bool
	LastPingAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
