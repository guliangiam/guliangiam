package aigateway

import (
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	RouteStatusActive   = "active"
	RouteStatusDisabled = "disabled"
)

// routeStatuses is the closed set the check and the 422 `allowed` metadata share.
var routeStatuses = []string{RouteStatusActive, RouteStatusDisabled}

type LlmRoute struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	AliasID        string     `json:"aliasId"`
	ModelID        string     `json:"modelId"`
	Weight         int        `json:"weight"`
	Priority       int        `json:"priority"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	DeletedAt      *time.Time `json:"deletedAt,omitempty"`
	Revision       int64      `json:"revision"`
}

func (r *LlmRoute) Validate() error {
	if r.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if r.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if r.AliasID == "" {
		return apperrors.NewInvalidArgument("aliasId is required")
	}
	if r.ModelID == "" {
		return apperrors.NewInvalidArgument("modelId is required")
	}
	if r.Weight <= 0 {
		r.Weight = 1
	}
	if r.Status == "" {
		r.Status = RouteStatusActive
	}
	if r.Status != RouteStatusActive && r.Status != RouteStatusDisabled {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid route status: %s", r.Status),
			map[string]any{"field": "status", "value": r.Status, "allowed": append([]string{}, routeStatuses...)},
		)
	}
	return nil
}
