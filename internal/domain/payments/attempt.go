package payments

import (
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	AttemptTypeCharge      = "charge"
	AttemptTypeQuery       = "query"
	AttemptTypeRefund      = "refund"
	AttemptTypeQueryRefund = "query_refund"
	AttemptTypeClose       = "close"
	AttemptTypeCallback    = "callback"
)

// attemptTypes / attemptStates are the single source for both the rules and
// the 422 `allowed` metadata.
var attemptTypes = []string{
	AttemptTypeCharge, AttemptTypeQuery, AttemptTypeRefund,
	AttemptTypeQueryRefund, AttemptTypeClose, AttemptTypeCallback,
}

var attemptStates = []string{
	string(AttemptStatePending), string(AttemptStateSucceeded), string(AttemptStateFailed),
	string(AttemptStateUnknown), string(AttemptStateRejected),
}

type PayAttempt struct {
	ID               string         `json:"id"`
	OrganizationID   string         `json:"organizationId"`
	PaymentID        string         `json:"paymentId"`
	RefundID         *string        `json:"refundId,omitempty"`
	Channel          string         `json:"channel"`
	AttemptType      string         `json:"attemptType"`
	AttemptNo        int            `json:"attemptNo"`
	ChannelOrderNo   *string        `json:"channelOrderNo,omitempty"`
	ChannelRequestID *string        `json:"channelRequestId,omitempty"`
	State            AttemptState   `json:"state"`
	Amount           *float64       `json:"amount,omitempty"`
	Currency         *string        `json:"currency,omitempty"`
	ErrorCode        *string        `json:"errorCode,omitempty"`
	ErrorMessage     *string        `json:"errorMessage,omitempty"`
	LatencyMs        *int           `json:"latencyMs,omitempty"`
	OccurredAt       time.Time      `json:"occurredAt"`
	RawResponse      map[string]any `json:"rawResponse,omitempty"`
}

func (a *PayAttempt) Validate() error {
	if a.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if a.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if a.PaymentID == "" {
		return apperrors.NewInvalidArgument("paymentId is required")
	}
	if a.Channel == "" {
		return apperrors.NewInvalidArgument("channel is required")
	}
	if a.AttemptType == "" {
		return apperrors.NewInvalidArgument("attemptType is required")
	}
	switch a.AttemptType {
	case AttemptTypeCharge, AttemptTypeQuery, AttemptTypeRefund,
		AttemptTypeQueryRefund, AttemptTypeClose, AttemptTypeCallback:
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid attemptType: %s", a.AttemptType),
			map[string]any{"field": "attemptType", "value": a.AttemptType, "allowed": append([]string{}, attemptTypes...)},
		)
	}
	if a.AttemptNo < 1 {
		a.AttemptNo = 1
	}
	if a.State == "" {
		a.State = AttemptStatePending
	}
	switch a.State {
	case AttemptStatePending, AttemptStateSucceeded, AttemptStateFailed, AttemptStateUnknown, AttemptStateRejected:
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid state: %s", a.State),
			map[string]any{"field": "state", "value": a.State, "allowed": append([]string{}, attemptStates...)},
		)
	}
	return nil
}
