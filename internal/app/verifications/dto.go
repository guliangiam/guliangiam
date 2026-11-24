package verifications

import (
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/verifications"
)

// VerificationDTO is the data transfer object returned by the service layer,
// decoupling consumers from the domain entity.
type VerificationDTO struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Type           string    `json:"type"`
	Recipient      string    `json:"recipient"`
	Code           string    `json:"code"`
	Status         string    `json:"status"`
	ExpiresAt      time.Time `json:"expiresAt"`
	CreatedAt      time.Time `json:"createdAt"`
}

func toVerificationDTO(v *domain.Verification) *VerificationDTO {
	if v == nil {
		return nil
	}
	// Code is intentionally left empty: read DTOs never carry OTP plaintext,
	// while write/verify paths keep using the domain entity directly.
	return &VerificationDTO{
		ID:             v.ID,
		OrganizationID: v.OrganizationID,
		Type:           v.Type,
		Recipient:      v.Recipient,
		Code:           "",
		Status:         v.Status,
		ExpiresAt:      v.ExpiresAt,
		CreatedAt:      v.CreatedAt,
	}
}
