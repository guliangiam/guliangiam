package deviceauthcodes

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
)

func toDomain(row deviceAuthCodeRow) *domain.DeviceAuthCode {
	return &domain.DeviceAuthCode{
		ID:              row.ID,
		DeviceCode:      row.DeviceCode,
		UserCode:        row.UserCode,
		ClientID:        row.ClientID,
		Scope:           row.Scope,
		ExpiresAt:       row.ExpiresAt,
		PollIntervalSec: row.PollIntervalSec,
		Verified:        row.Verified,
		VerifiedAt:      row.VerifiedAt,
		UserID:          row.UserID,
		Username:        row.Username,
		OrganizationID:  row.OrganizationID,
		LastPolledAt:    row.LastPolledAt,
		SlowDownCount:   row.SlowDownCount,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		Revision:        row.Revision,
	}
}
