// Package scim 实现 SCIM v2 出站 Provisioning 的 DeliveryHandler + HTTP 客户端。
// 零外部依赖，仅用标准库 net/http。
package scim

import (
	"context"

	"io"
	"net/http"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// maxResponseBodyBytes 限制单个 SCIM 响应体大小，防止异常上游无限消耗内存。
const maxResponseBodyBytes int64 = 1 << 20 // 1 MiB

// Client 是 SCIM v2 出站 HTTP 客户端。
type Client struct {
	httpClient    *http.Client
	baseURL       string
	bearer        string
	maxRatePerMin int // 0=不限

	// 令牌桶：channel-based。容量 = maxRatePerMin，每 60s 补充满。
	bucket chan struct{}
}

// NewClient 构造 Client。
// timeout 单位 ms，<=0 时使用 5000ms 默认值。
// maxRatePerMin=0 表示不限速。
func NewClient(baseURL, bearer string, timeout, maxRatePerMin int) *Client {
	if timeout <= 0 {
		timeout = 5000
	}
	c := &Client{
		httpClient:    scimHTTPClient(timeout),
		baseURL:       strings.TrimRight(baseURL, "/"),
		bearer:        bearer,
		maxRatePerMin: maxRatePerMin,
	}
	if maxRatePerMin > 0 {
		c.bucket = make(chan struct{}, maxRatePerMin)
		// 预填充令牌
		for i := 0; i < maxRatePerMin; i++ {
			c.bucket <- struct{}{}
		}
		// 后台补充：每 60s 补满一次（简单的 fixed-interval refill）
		recover.SafeGo("scim-outbound-refill", c.refillLoop)
	}
	return c
}

// refillLoop 每 60s 补满令牌桶。goroutine 随进程退出而终止。
func (c *Client) refillLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		for i := 0; i < c.maxRatePerMin; i++ {
			select {
			case c.bucket <- struct{}{}:
			default:
				// 桶未空，跳过（避免阻塞）
			}
		}
	}
}

// Do 执行 SCIM HTTP 调用。
// 返回 HTTP status（即使 >=400 也返回，由上层决定是否重试）。
// 返回的 error 仅用于连接层故障（DNS/timeout/TLS）——HTTP 4xx/5xx 不 return error。
// 按 ADR-008 Outbox 重试语义：
//   - 连接层 error → Worker 退避重试
//   - HTTP 400/401 → 死信（业务拒绝）
//   - HTTP 429/500 → 退避重试
func (c *Client) Do(ctx context.Context, method, path string, body []byte) (status int, respBody []byte, err error) {
	// 速率限制
	if c.bucket != nil {
		select {
		case <-c.bucket:
		case <-ctx.Done():
			return 0, nil, ctx.Err()
		}
	}

	fullURL := c.joinPath(path)

	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = strings.NewReader(string(body))
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return 0, nil, apperrors.WrapInternal(err, "scim client: build request")
	}
	req.Header.Set("Content-Type", "application/scim+json")
	req.Header.Set("Accept", "application/scim+json")
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, apperrors.WrapUnavailable(err, "scim client: http do")
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes+1))
	if readErr != nil {
		// 即使 body 读失败，也把 status 返回给上层
		return resp.StatusCode, nil, apperrors.WrapUnavailable(readErr, "scim client: read response")
	}
	if int64(len(respBody)) > maxResponseBodyBytes {
		return resp.StatusCode, nil, apperrors.NewUnavailablef("scim client: response body exceeds %d bytes", maxResponseBodyBytes)
	}

	return resp.StatusCode, respBody, nil
}

// joinPath 拼接 baseURL + path。
// baseURL 结尾 `/` 可选；path 开头 `/` 可选；都能正确拼接。
func (c *Client) joinPath(path string) string {
	p := strings.TrimLeft(path, "/")
	if p == "" {
		return c.baseURL
	}
	return c.baseURL + "/" + p
}

// scimHTTPClient returns the outbound HTTP client. SCIM targets are normally
// external SaaS endpoints, so SSRF protection is the default. On-prem
// connectors that intentionally point at a private network can opt in with
// SCIM_ALLOW_PRIVATE_TARGETS=true (documented trade-off in security-model S7).
func scimHTTPClient(timeoutMs int) *http.Client {
	timeout := time.Duration(timeoutMs) * time.Millisecond
	if config.SCIMAllowPrivateTargets() {
		return &http.Client{Timeout: timeout}
	}
	return ssrf.SafeHTTPClient(timeout)
}
