package aigateway

import (
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type LlmPriceVersion struct {
	ID               string     `json:"id"`
	OrganizationID   string     `json:"organizationId"`
	ModelID          string     `json:"modelId"`
	VersionNo        int        `json:"versionNo"`
	InputPricePer1M  float64    `json:"inputPricePer1M"`
	OutputPricePer1M float64    `json:"outputPricePer1M"`
	Currency         string     `json:"currency"`
	EffectiveFrom    time.Time  `json:"effectiveFrom"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"`
	Revision         int64      `json:"revision"`
}

// CalculateCost computes the charge for a single call. Currencies are assumed
// to match the version's declared currency; cross-currency conversion happens
// one layer above. Zero-token calls still produce a zero-cost UsageLog row
// so metering and accounting stay consistent.
func (v *LlmPriceVersion) CalculateCost(inputTokens, outputTokens int) float64 {
	inputCost := (float64(inputTokens) / 1_000_000) * v.InputPricePer1M
	outputCost := (float64(outputTokens) / 1_000_000) * v.OutputPricePer1M
	return inputCost + outputCost
}

func (v *LlmPriceVersion) Validate() error {
	if v.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if v.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if v.ModelID == "" {
		return apperrors.NewInvalidArgument("modelId is required")
	}
	if v.InputPricePer1M < 0 || v.OutputPricePer1M < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "prices must be non-negative", map[string]any{"field": "prices", "min": 0})
	}
	if v.Currency == "" {
		v.Currency = currencyUSD
	}
	return nil
}
