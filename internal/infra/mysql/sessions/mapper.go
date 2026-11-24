package sessions

import (
	"database/sql"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
)

// toSession 把 sessionRow（MySQL SQL 扫描结果）转成 domain.Session。
// 与 PG 版 toSession 字段级一一对应（全链路契约同步）。
func toSession(row sessionRow) (*domain.Session, error) {
	session := &domain.Session{
		ID:                  row.ID,
		UserID:              row.UserID,
		Username:            row.Username,
		OrganizationID:      row.OrganizationID,
		OrganizationSlug:    row.OrganizationSlug,
		ApplicationID:       row.ApplicationID,
		ApplicationSlug:     row.ApplicationSlug,
		SessionState:        domain.State(row.SessionState),
		Sid:                 nullString(row.Sid),
		IPAddress:           nullString(row.IPAddress),
		UserAgent:           nullString(row.UserAgent),
		ExpiresAt:           row.ExpiresAt,
		RevokedAt:           nullTimePtr(row.RevokedAt),
		CreatedAt:           row.CreatedAt,
		LastSeenAt:          row.LastSeenAt,
		UpdatedAt:           row.UpdatedAt,
		Revision:            row.Revision,
		AuthenticationLevel: row.AuthenticationLevel,
		AssuranceLevel:      row.AssuranceLevel,
		IsPrivileged:        row.IsPrivileged,
		LifecycleID:         row.LifecycleID,
		TicketID:            row.TicketID,
		DeviceInfo:          map[string]any{},
	}

	if len(row.DeviceInfo) > 0 {
		if err := json.Unmarshal(row.DeviceInfo, &session.DeviceInfo); err != nil {
			return nil, err
		}
	}
	if session.DeviceInfo == nil {
		session.DeviceInfo = map[string]any{}
	}

	return session, nil
}

// nullString 把 sql.NullString 展平，MySQL VARCHAR NULL → ""。
func nullString(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

// nullTimePtr 把 sql.NullTime 转成 *time.Time。MySQL TIMESTAMP NULL → nil。
func nullTimePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	t := nt.Time
	return &t
}
