package verifications

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/verifications"
)

func toVerification(row verificationRow) domain.Verification {
	return domain.Verification{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Type:           row.Type,
		Recipient:      row.Recipient,
		Code:           row.Code,
		Status:         row.Status,
		ExpiresAt:      row.ExpiresAt,
		CreatedAt:      row.CreatedAt,
	}
}
