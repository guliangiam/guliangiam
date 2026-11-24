// SPI 位置：Domain 层 — 见 ADR-035。
// ChannelGateway 是支付渠道的统一能力抽象。编排层（App）只依赖此 Port，
// 不感知渠道细节。Infra 层 13 个适配器实现此接口。
package payments

import (
	"context"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ErrorCategory is the closed classification of a channel failure. Producers
// (the infra adapters) set it once at the error point; callers branch on
// Retryable instead of re-deducing retry semantics from the code or message.
//
// The set refines the three-way Retryable/NonRetryable/Unknown vocabulary in
// docs/design/commerce/payment.md §16: every value below maps to exactly one
// of those three answers, and Retryable is the single source of truth for it.
type ErrorCategory int

const (
	ErrorCategoryUnknown              ErrorCategory = iota
	ErrorCategoryTransient                          // 5xx, 网络超时, 连接失败 → 可重试
	ErrorCategoryRateLimited                        // 上游 429 / 配额 → 可重试, 遵循 Retry-After
	ErrorCategoryAuth                               // 上游 401/403：渠道凭据被拒 → 不可重试
	ErrorCategoryPermanent                          // 其余 4xx / 请求或签名错误 → 不可重试
	ErrorCategoryConfig                             // 配置缺失 / 解密失败 → 不可重试
	ErrorCategoryChannelOrderNotFound               // 渠道无此单据 → 不可重试
)

// Deprecated aliases kept so existing readers (workers, tests) compile. They
// are the same values as the finer-grained names above.
const (
	// Deprecated: use ErrorCategoryTransient.
	ErrorCategoryRetryable = ErrorCategoryTransient
	// Deprecated: use ErrorCategoryPermanent.
	ErrorCategoryNonRetryable = ErrorCategoryPermanent
)

// Retryable reports whether the category allows another attempt (possibly
// after the RetryAfter delay). It is the only retry predicate for channel
// failures; comparing against a single constant would miss RateLimited.
func (c ErrorCategory) Retryable() bool {
	switch c {
	case ErrorCategoryTransient, ErrorCategoryRateLimited:
		return true
	default:
		return false
	}
}

// String returns the stable lower_snake identifier used in logs and tests.
func (c ErrorCategory) String() string {
	switch c {
	case ErrorCategoryTransient:
		return "transient"
	case ErrorCategoryRateLimited:
		return "rate_limited"
	case ErrorCategoryAuth:
		return "auth"
	case ErrorCategoryPermanent:
		return "permanent"
	case ErrorCategoryConfig:
		return "config"
	case ErrorCategoryChannelOrderNotFound:
		return "channel_order_not_found"
	default:
		return "unknown"
	}
}

type ChannelError struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Category   ErrorCategory  `json:"category"`
	RawStatus  string         `json:"rawStatus,omitempty"`
	RetryAfter *time.Duration `json:"retryAfter,omitempty"`
	// Cause keeps the adapter-level error chain (status, transport error,
	// wrapped SDK error) reachable via errors.Is/errors.As without leaking it
	// into the JSON contract.
	Cause error `json:"-"`
}

func (e *ChannelError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

// Unwrap exposes the original adapter error so callers can inspect the chain
// (e.g. a wrapped net.Error or a platform error) without parsing the message.
func (e *ChannelError) Unwrap() error { return e.Cause }

// ChannelMetadata describes the adapter itself. The orchestration layer uses
// this at bootstrap to decide which provider maps to which gateway.
type ChannelMetadata struct {
	Channel           string `json:"channel"`
	SupportsCharge    bool   `json:"supportsCharge"`
	SupportsRefund    bool   `json:"supportsRefund"`
	SupportsQuery     bool   `json:"supportsQuery"`
	SupportsClose     bool   `json:"supportsClose"`
	SupportsCallback  bool   `json:"supportsCallback"`
	SupportsStatement bool   `json:"supportsStatement"` // reconciliation G11
}

type ChargeRequest struct {
	PaymentID      string  `json:"paymentId"`
	OrganizationID string  `json:"organizationId"`
	OutTradeNo     string  `json:"outTradeNo"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	Subject        string  `json:"subject,omitempty"`
	ReturnURL      string  `json:"returnUrl,omitempty"`
	NotifyURL      string  `json:"notifyUrl,omitempty"`
	ClientIP       string  `json:"clientIp,omitempty"`
}

type AttemptState string

const (
	AttemptStatePending   AttemptState = "pending"
	AttemptStateSucceeded AttemptState = "succeeded"
	AttemptStateFailed    AttemptState = "failed"
	AttemptStateUnknown   AttemptState = "unknown"
	AttemptStateRejected  AttemptState = "rejected"
)

type ChargeResult struct {
	State          AttemptState  `json:"state"`
	ChannelOrderNo string        `json:"channelOrderNo,omitempty"`
	RequestID      string        `json:"requestId,omitempty"`
	PayURL         string        `json:"payUrl,omitempty"`
	QRCode         string        `json:"qrCode,omitempty"`
	ErrorCode      string        `json:"errorCode,omitempty"`
	ErrorMessage   string        `json:"errorMessage,omitempty"`
	ErrorCategory  ErrorCategory `json:"-"`
}

type QueryRequest struct {
	PaymentID      string `json:"paymentId"`
	ChannelOrderNo string `json:"channelOrderNo,omitempty"`
	OutTradeNo     string `json:"outTradeNo,omitempty"`
}

type QueryResult struct {
	State          AttemptState `json:"state"`
	ChannelOrderNo string       `json:"channelOrderNo,omitempty"`
	Amount         *float64     `json:"amount,omitempty"`
	ErrorCode      string       `json:"errorCode,omitempty"`
}

type RefundRequest struct {
	PaymentID      string  `json:"paymentId"`
	OutTradeNo     string  `json:"outTradeNo"`
	OutRefundNo    string  `json:"outRefundNo"`
	ChannelOrderNo string  `json:"channelOrderNo,omitempty"`
	Amount         float64 `json:"amount"`
	Reason         string  `json:"reason,omitempty"`
}

type RefundResult struct {
	State           AttemptState `json:"state"`
	ChannelRefundNo string       `json:"channelRefundNo,omitempty"`
	RequestID       string       `json:"requestId,omitempty"`
	ErrorCode       string       `json:"errorCode,omitempty"`
	ErrorMessage    string       `json:"errorMessage,omitempty"`
}

type QueryRefundRequest struct {
	PaymentID       string `json:"paymentId"`
	OutRefundNo     string `json:"outRefundNo"`
	ChannelRefundNo string `json:"channelRefundNo,omitempty"`
}

type QueryRefundResult struct {
	State     AttemptState `json:"state"`
	Amount    *float64     `json:"amount,omitempty"`
	ErrorCode string       `json:"errorCode,omitempty"`
}

type CloseRequest struct {
	PaymentID      string `json:"paymentId"`
	OutTradeNo     string `json:"outTradeNo"`
	ChannelOrderNo string `json:"channelOrderNo,omitempty"`
}

type CloseResult struct {
	State AttemptState `json:"state"`
}

// ChannelConfig is the fully decrypted config an adapter needs to call the
// vendor. The envelope-decryption step happens one layer above — ChannelGateway
// receives a plain config map and only cares about the values being present.
type ChannelConfig struct {
	AppID         string
	MerchantID    string
	PrivateKey    string
	NotifyURL     string
	WebhookSecret string
	Extra         map[string]any
}

type CallbackRequest struct {
	Channel  string              `json:"channel"`
	Payload  []byte              `json:"payload"`
	Headers  map[string][]string `json:"headers"`
	ClientIP string              `json:"clientIp"`
}

// CallbackEvent is the normalised, verified event the adapter emits. The
// pipeline's stage 4 writes it to pay_callback_receipts.raw_event and stage 5
// uses it to advance the payment state machine.
type CallbackEvent struct {
	EventKey       string         `json:"eventKey"`
	OutTradeNo     string         `json:"outTradeNo"`
	ChannelOrderNo string         `json:"channelOrderNo,omitempty"`
	RawStatus      string         `json:"rawStatus"`
	Amount         *float64       `json:"amount,omitempty"`
	State          AttemptState   `json:"state"`
	Raw            map[string]any `json:"raw,omitempty"`
}

// StatementRequest asks an adapter to pull a reconciliation statement for a
// specific batch date (usually T-1). From/To are half-open: [BatchDate 00:00,
// BatchDate+1 00:00). The adapter maps this to vendor-specific API params.
type StatementRequest struct {
	OrganizationID string    `json:"organizationId"`
	Channel        string    `json:"channel"`
	BatchDate      time.Time `json:"batchDate"`
}

// StatementLine is one row from a channel-side statement. Reconciliation
// compares each line against a local payment / refund record.
type StatementLine struct {
	OutTradeNo      string         `json:"outTradeNo"`
	ChannelOrderNo  string         `json:"channelOrderNo,omitempty"`
	Amount          float64        `json:"amount"`
	State           string         `json:"state"` // vendor-normalised → AttemptState
	Type            string         `json:"type"`  // "charge" | "refund"
	ChannelRefundNo string         `json:"channelRefundNo,omitempty"`
	OccurredAt      time.Time      `json:"occurredAt"`
	Raw             map[string]any `json:"raw,omitempty"`
}

// ChannelGateway is the SPI. 9 methods cover the five-stage callback
// pipeline, active query, refund flow, close, signature verification,
// config self-test, and statement download for the daily reconciliation
// worker (G11). ADR-035 places the interface in the Domain layer — Infra
// implements it per vendor; App orchestration only knows the Port.
type ChannelGateway interface {
	Metadata() ChannelMetadata
	Charge(ctx context.Context, req ChargeRequest) (*ChargeResult, *ChannelError)
	Query(ctx context.Context, req QueryRequest) (*QueryResult, *ChannelError)
	Refund(ctx context.Context, req RefundRequest) (*RefundResult, *ChannelError)
	QueryRefund(ctx context.Context, req QueryRefundRequest) (*QueryRefundResult, *ChannelError)
	Close(ctx context.Context, req CloseRequest) (*CloseResult, *ChannelError)
	ParseCallback(ctx context.Context, req CallbackRequest) (*CallbackEvent, error)
	VerifyConfig(ctx context.Context, config ChannelConfig) error
	DownloadStatement(ctx context.Context, req StatementRequest) ([]StatementLine, *ChannelError)
}

// ErrChannelUnavailable is returned when no adapter is registered for the
// requested channel. The orchestration layer should surface this as a 503.
var ErrChannelUnavailable = apperrors.New(apperrors.CodePaymentChannelUnavailable, "no adapter registered for requested channel")
