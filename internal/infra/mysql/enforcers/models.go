package enforcers

import (
	"database/sql"
	"time"
)

// enforcerRow 是 MySQL enforcers 表的扫描目标。12 列。
type enforcerRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	ModelID        sql.NullString // nullable FK → models(id) ON DELETE SET NULL
	AdapterID      sql.NullString // nullable FK → adapters(id) ON DELETE SET NULL
	IsEnabled      bool
	Owner          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

func nullOrString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
