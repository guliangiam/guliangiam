package aigateway

import (
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	ModelStatusActive     = "active"
	ModelStatusDeprecated = "deprecated"
)

type LlmModel struct {
	ID                string         `json:"id"`
	OrganizationID    string         `json:"organizationId"`
	ProviderID        string         `json:"providerId"`
	ProviderModelName string         `json:"providerModelName"`
	DisplayName       string         `json:"displayName,omitempty"`
	Capabilities      map[string]any `json:"capabilities,omitempty"`
	MaxContextTokens  *int           `json:"maxContextTokens,omitempty"`
	MaxOutputTokens   *int           `json:"maxOutputTokens,omitempty"`
	Status            string         `json:"status"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	DeletedAt         *time.Time     `json:"deletedAt,omitempty"`
	Revision          int64          `json:"revision"`
}

func (m *LlmModel) Validate() error {
	if m.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if m.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if m.ProviderID == "" {
		return apperrors.NewInvalidArgument("providerId is required")
	}
	if m.ProviderModelName == "" {
		return apperrors.NewInvalidArgument("providerModelName is required")
	}
	if m.Status == "" {
		m.Status = ModelStatusActive
	}
	if m.Status != ModelStatusActive && m.Status != ModelStatusDeprecated {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid model status: %s", m.Status),
			map[string]any{"field": "status", "value": m.Status, "allowed": []string{ModelStatusActive, ModelStatusDeprecated}},
		)
	}
	return nil
}

func (m *LlmModel) CanAcceptTraffic() bool {
	return m.Status == ModelStatusActive && m.DeletedAt == nil
}
