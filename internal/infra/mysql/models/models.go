package models

import "time"

// modelRow 是 MySQL models 表的扫描目标。11 列。
type modelRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	ModelText      string
	Owner          string
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
