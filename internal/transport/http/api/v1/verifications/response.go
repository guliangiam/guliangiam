package verifications

import (
	"time"

	appverifications "github.com/guliangiam/guliangiam/internal/app/verifications"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type VerificationResponse struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Type           string    `json:"type"`
	Recipient      string    `json:"recipient"`
	Code           string    `json:"code"`
	Status         string    `json:"status"`
	ExpiresAt      time.Time `json:"expiresAt"`
	CreatedAt      time.Time `json:"createdAt"`
}

func toResponse(verification *appverifications.VerificationDTO) *VerificationResponse {
	// Code is intentionally left empty: read responses never carry OTP plaintext.
	return &VerificationResponse{
		ID:             verification.ID,
		OrganizationID: verification.OrganizationID,
		Type:           verification.Type,
		Recipient:      verification.Recipient,
		Code:           "",
		Status:         verification.Status,
		ExpiresAt:      verification.ExpiresAt,
		CreatedAt:      verification.CreatedAt,
	}
}

type ListVerificationsResponse = presenter.ListResponse[*appverifications.VerificationDTO]

type SendVerificationCodeResponse struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Recipient string    `json:"recipient"`
	ExpiresAt time.Time `json:"expiresAt"`
}
