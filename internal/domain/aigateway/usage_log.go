package aigateway

import (
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	UsageStatusSucceeded = "succeeded"
	UsageStatusFailed    = "failed"
	UsageStatusTimeout   = "timeout"
)

// protocolFaces / usageStatuses are the closed sets the checks and the 422
// `allowed` metadata share.
var protocolFaces = []string{"openai", "anthropic", "gemini"}

var usageStatuses = []string{UsageStatusSucceeded, UsageStatusFailed, UsageStatusTimeout}

type UsageLog struct {
	ID                string    `json:"id"`
	OrganizationID    string    `json:"organizationId"`
	APIKeyID          *string   `json:"apiKeyId,omitempty"`
	AliasID           string    `json:"aliasId"`
	ProviderID        string    `json:"providerId"`
	ModelID           string    `json:"modelId"`
	PriceVersionID    *string   `json:"priceVersionId,omitempty"`
	ProtocolFace      string    `json:"protocolFace"`
	AttemptNo         int       `json:"attemptNo"`
	InputTokens       int       `json:"inputTokens"`
	OutputTokens      int       `json:"outputTokens"`
	TotalTokens       int       `json:"totalTokens"`
	CostAmount        float64   `json:"costAmount"`
	Currency          string    `json:"currency"`
	Status            string    `json:"status"`
	ErrorCode         *string   `json:"errorCode,omitempty"`
	TTFTMs            *int      `json:"ttftMs,omitempty"`
	TotalLatencyMs    *int      `json:"totalLatencyMs,omitempty"`
	UpstreamRequestID *string   `json:"upstreamRequestId,omitempty"`
	FailoverFrom      *string   `json:"failoverFrom,omitempty"`
	RequestID         string    `json:"requestId"`
	CreatedAt         time.Time `json:"createdAt"`
}

func (u *UsageLog) Validate() error {
	if u.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if u.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if u.AliasID == "" || u.ProviderID == "" || u.ModelID == "" {
		return apperrors.NewInvalidArgument("aliasId, providerId and modelId are required")
	}
	if u.RequestID == "" {
		return apperrors.NewInvalidArgument("requestId is required")
	}
	if u.ProtocolFace != "openai" && u.ProtocolFace != "anthropic" && u.ProtocolFace != "gemini" {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid protocolFace: %s", u.ProtocolFace),
			map[string]any{"field": "protocolFace", "value": u.ProtocolFace, "allowed": append([]string{}, protocolFaces...)},
		)
	}
	switch u.Status {
	case UsageStatusSucceeded, UsageStatusFailed, UsageStatusTimeout:
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid usage status: %s", u.Status),
			map[string]any{"field": "status", "value": u.Status, "allowed": append([]string{}, usageStatuses...)},
		)
	}
	if u.InputTokens < 0 || u.OutputTokens < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "token counts must be non-negative", map[string]any{"field": "tokenCounts", "min": 0})
	}
	if u.CostAmount < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "costAmount must be non-negative", map[string]any{"field": "costAmount", "value": u.CostAmount, "min": 0})
	}
	if u.Currency == "" {
		u.Currency = currencyUSD
	}
	if u.AttemptNo < 1 {
		u.AttemptNo = 1
	}
	return nil
}
