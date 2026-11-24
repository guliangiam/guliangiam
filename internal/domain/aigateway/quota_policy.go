package aigateway

import (
	"slices"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	QuotaStatusActive   = "active"
	QuotaStatusDisabled = "disabled"
)

// resetCycles is the closed set ResetCycle accepts; the check and the 422
// `allowed` metadata read the same slice.
var resetCycles = []string{"daily", "monthly"}

// quotaScopes / limitTypes are the closed sets Validate accepts; the checks and
// the 422 `allowed` metadata read the same slices.
var quotaScopes = []string{QuotaScopeOrganization, QuotaScopeAPIKey, QuotaScopeAlias}

var limitTypes = []string{LimitTypeRPM, LimitTypeTPM, LimitTypeCost}

const (
	QuotaScopeOrganization = "organization"
	QuotaScopeAPIKey       = "api_key"
	QuotaScopeAlias        = "alias"
)

const (
	LimitTypeRPM  = "rpm"
	LimitTypeTPM  = "tpm"
	LimitTypeCost = "cost"
)

type LlmQuotaPolicy struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Scope          string     `json:"scope"`
	ScopeRef       *string    `json:"scopeRef,omitempty"`
	LimitType      string     `json:"limitType"`
	LimitValue     float64    `json:"limitValue"`
	ResetCycle     string     `json:"resetCycle"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	DeletedAt      *time.Time `json:"deletedAt,omitempty"`
	Revision       int64      `json:"revision"`
}

// CheckLimit returns ErrQuotaExceeded (or its wrapped version) if current
// usage >= the policy's ceiling. Callers are responsible for reading the
// actual usage from Redis / metering — this method only compares numbers.
func (p *LlmQuotaPolicy) CheckLimit(currentUsage float64) error {
	if p.Status != QuotaStatusActive {
		return nil // disabled policy does not enforce
	}
	if currentUsage >= p.LimitValue {
		return apperrors.NewTooManyRequestsf("quota exceeded: %s %s limit %.2f, current %.2f",
			p.Scope, p.LimitType, p.LimitValue, currentUsage)
	}
	return nil
}

func (p *LlmQuotaPolicy) Validate() error {
	if p.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if p.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	switch p.Scope {
	case QuotaScopeOrganization, QuotaScopeAPIKey, QuotaScopeAlias:
		// valid
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"invalid quota scope: "+p.Scope,
			map[string]any{"field": "scope", "value": p.Scope, "allowed": append([]string{}, quotaScopes...)},
		)
	}
	switch p.LimitType {
	case LimitTypeRPM, LimitTypeTPM, LimitTypeCost:
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"invalid limit type: "+p.LimitType,
			map[string]any{"field": "limitType", "value": p.LimitType, "allowed": append([]string{}, limitTypes...)},
		)
	}
	if p.LimitValue <= 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "limitValue must be positive", map[string]any{"field": "limitValue", "value": p.LimitValue, "min": 1})
	}
	if !slices.Contains(resetCycles, p.ResetCycle) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "resetCycle must be daily or monthly", map[string]any{"field": "resetCycle", "value": p.ResetCycle, "allowed": append([]string{}, resetCycles...)})
	}
	if p.Status == "" {
		p.Status = QuotaStatusActive
	}
	return nil
}
