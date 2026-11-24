package qrcodesessions

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
)

func toDomain(row sessionRow) *domain.Session {
	return &domain.Session{
		ID:             row.ID,
		CodeID:         row.CodeID,
		Code:           row.Code,
		OrganizationID: row.OrganizationID,
		ApplicationID:  row.ApplicationID,
		ExpiresAt:      row.ExpiresAt,
		Status:         row.Status,
		UserID:         row.UserID,
		Username:       row.Username,
		IPAddress:      row.IPAddress,
		UserAgent:      row.UserAgent,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
}
