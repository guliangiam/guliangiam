package aigateway

import (
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	AliasStatusActive   = "active"
	AliasStatusDisabled = "disabled"
)

type LlmModelAlias struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Alias          string     `json:"alias"`
	DisplayName    string     `json:"displayName,omitempty"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	DeletedAt      *time.Time `json:"deletedAt,omitempty"`
	Revision       int64      `json:"revision"`
}

func (a *LlmModelAlias) Validate() error {
	if a.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if a.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if a.Alias == "" {
		return apperrors.NewInvalidArgument("alias is required")
	}
	if a.Status == "" {
		a.Status = AliasStatusActive
	}
	if a.Status != AliasStatusActive && a.Status != AliasStatusDisabled {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid alias status: %s", a.Status),
			map[string]any{"field": "status", "value": a.Status, "allowed": []string{AliasStatusActive, AliasStatusDisabled}},
		)
	}
	return nil
}

func (a *LlmModelAlias) IsAvailable() bool {
	return a.Status == AliasStatusActive && a.DeletedAt == nil
}
