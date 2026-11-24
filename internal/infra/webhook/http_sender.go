package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	webhookdomain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
)

const (
	// SignatureHeader is the HTTP header for webhook HMAC signature.
	// Format: t=<unix_sec>,v1=<hex_hmac_sha256(t + '.' + body)>
	SignatureHeader = "GuliangIAM-Signature"
	// TimestampHeader is the timestamp alone, for receivers that want
	// to check ±5min before parsing the signature string.
	TimestampHeader = "GuliangIAM-Timestamp"
)

var _ webhookdomain.DeliverySender = (*HTTPDeliverySender)(nil)

type HTTPDeliverySender struct {
	client  *http.Client
	timeout time.Duration
	nowFunc func() time.Time
}

// NewHTTPDeliverySender 创建一个带 SSRF 防护 + HMAC 签名的 webhook HTTP sender。
//
// 出站连接经过：
//   - scheme 白名单（http/https only）
//   - 解析后 IP 校验（禁私有/链路本地/回环/metadata）
//   - 重定向每跳重新校验
//
// 签名（ADR-018 / S18）：
//   - 空 secret → 不签名，只透传 headers
//   - 非空 secret → 生成 GuliangIAM-Signature: t=<unix_sec>,v1=<hex>
//   - GuliangIAM-Timestamp: <unix_sec>
//   - 签名范围：`<timestamp>.<body>`
//   - 接收方校验签名 + 时间戳 ±5min 防重放（出站方只负责生成）
func NewHTTPDeliverySender(client *http.Client, timeout time.Duration) *HTTPDeliverySender {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if client == nil {
		client = ssrf.SafeHTTPClient(timeout)
	}
	return &HTTPDeliverySender{client: client, timeout: timeout, nowFunc: time.Now}
}

func (s *HTTPDeliverySender) Send(ctx context.Context, targetURL string, headers map[string]string, payload []byte, secret string) webhookdomain.DeliveryResult {
	if err := ssrf.ValidateURL(targetURL); err != nil {
		return webhookdomain.DeliveryResult{Error: err}
	}

	reqCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, targetURL, bytes.NewReader(payload))
	if err != nil {
		return webhookdomain.DeliveryResult{Error: err}
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// HMAC 签名（空 secret → 跳过）
	if secret != "" {
		timestamp := s.nowFunc().Unix()
		sig := computeSignature(secret, timestamp, payload)
		req.Header.Set(SignatureHeader, fmt.Sprintf("t=%d,v1=%s", timestamp, sig))
		req.Header.Set(TimestampHeader, fmt.Sprintf("%d", timestamp))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return webhookdomain.DeliveryResult{Error: err}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return webhookdomain.DeliveryResult{
		StatusCode:   resp.StatusCode,
		ResponseBody: string(respBody),
	}
}

// computeSignature 生成 HMAC-SHA256 签名。
//
// 签名范围：`<timestamp>.<body>`，timestamp 为 Unix 秒（t= 字段的值）。
// 接收方用同一 secret 对 `t + "." + body` 重算并比对；时间戳 ±5min 内有效
// （防重放窗口由接收方校验）。
func computeSignature(secret string, timestamp int64, payload []byte) string {
	signingStr := fmt.Sprintf("%d.%s", timestamp, string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingStr))
	return hex.EncodeToString(mac.Sum(nil))
}
