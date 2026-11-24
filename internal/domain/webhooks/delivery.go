package webhooks

import (
	"context"
)

// DeliveryResult contains the result of a webhook delivery attempt.
type DeliveryResult struct {
	StatusCode   int
	ResponseBody string
	Error        error
}

// DeliverySender is the port interface for sending webhook payloads.
// Implementations live in the infra layer (e.g., HTTP-based delivery).
//
// secret 参数用于 HMAC 签名（ADR-018 / security-readiness S18）。
// 空 secret → 不签名；有 secret → 生成 GuliangIAM-Signature: t=<unix_sec>,v1=<hex>
// 与 GuliangIAM-Timestamp: <unix_sec>（签名范围 `<t>.<body>`，HMAC-SHA256，
// 实现见 internal/infra/webhook/http_sender.go）。
// 签名时间戳 t 用于接收方 ±5min 防重放校验（出站方只负责生成，接收方负责窗口检查）。
type DeliverySender interface {
	Send(ctx context.Context, url string, headers map[string]string, payload []byte, secret string) DeliveryResult
}
