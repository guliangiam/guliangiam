package payments

import (
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// process_state 是五阶段回调管线的流水线状态：
//
//	received → normalized → accepted / duplicated
//
// 验签与归一化在 adapter.ParseCallback 内同一步完成，管线不单独落库 verified
// 中间态（verified 保留为历史兼容态，现行管线不产生）；任一非终态可转 rejected。
const (
	ReceiptStateReceived   = "received"
	ReceiptStateVerified   = "verified"
	ReceiptStateRejected   = "rejected"
	ReceiptStateDuplicated = "duplicated"
	ReceiptStateNormalized = "normalized"
	ReceiptStateAccepted   = "accepted"
)

var validReceiptTransitions = map[string][]string{
	ReceiptStateReceived:   {ReceiptStateNormalized, ReceiptStateRejected},
	ReceiptStateVerified:   {ReceiptStateNormalized, ReceiptStateRejected},
	ReceiptStateNormalized: {ReceiptStateAccepted, ReceiptStateDuplicated, ReceiptStateRejected},
	// rejected, duplicated, accepted are terminal
}

// receiptStates is the closed set the check and the 422 `allowed` metadata share.
var receiptStates = []string{
	ReceiptStateReceived, ReceiptStateVerified, ReceiptStateRejected,
	ReceiptStateDuplicated, ReceiptStateNormalized, ReceiptStateAccepted,
}

type CallbackReceipt struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organizationId"`
	Channel        string         `json:"channel"`
	PayloadSHA256  string         `json:"payloadSha256"`
	Payload        []byte         `json:"-"`
	Headers        map[string]any `json:"headers,omitempty"`
	ReceivedAt     time.Time      `json:"receivedAt"`
	ClientIP       *string        `json:"clientIp,omitempty"`
	ProcessState   string         `json:"processState"`
	EventKey       *string        `json:"eventKey,omitempty"`
	OutTradeNo     *string        `json:"outTradeNo,omitempty"`
	RawEvent       map[string]any `json:"rawEvent,omitempty"`
	ErrorMessage   *string        `json:"errorMessage,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	Revision       int64          `json:"revision"`
}

func (c *CallbackReceipt) Validate() error {
	if c.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if c.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if c.Channel == "" {
		return apperrors.NewInvalidArgument("channel is required")
	}
	if c.PayloadSHA256 == "" {
		return apperrors.NewInvalidArgument("payloadSha256 is required")
	}
	if len(c.Payload) == 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "payload must not be empty", map[string]any{"field": "payload", "min": 1})
	}
	if c.ProcessState == "" {
		c.ProcessState = ReceiptStateReceived
	}
	switch c.ProcessState {
	case ReceiptStateReceived, ReceiptStateVerified, ReceiptStateRejected,
		ReceiptStateDuplicated, ReceiptStateNormalized, ReceiptStateAccepted:
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid processState: %s", c.ProcessState),
			map[string]any{"field": "processState", "value": c.ProcessState, "allowed": append([]string{}, receiptStates...)},
		)
	}
	return nil
}

func (c *CallbackReceipt) CanTransitionTo(newState string) bool {
	for _, s := range validReceiptTransitions[c.ProcessState] {
		if s == newState {
			return true
		}
	}
	return false
}

func (c *CallbackReceipt) TransitionTo(newState string) error {
	if newState == c.ProcessState {
		return nil
	}
	if !c.CanTransitionTo(newState) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot move receipt from %s to %s", c.ProcessState, newState),
			map[string]any{"field": "processState", "value": c.ProcessState, "allowed": append([]string{}, validReceiptTransitions[c.ProcessState]...)},
		)
	}
	c.ProcessState = newState
	return nil
}
