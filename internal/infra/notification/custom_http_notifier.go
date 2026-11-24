package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// CustomHTTPNotifier sends notifications to a configurable HTTP endpoint.
// Endpoint 来自用户配置，默认走 SSRF 防护通道。
//
// Calibration (OPEN-API-75 P6, 2026-11-14)：自定义端点无外部厂商契约可校准；载荷为
// 本仓库自定 JSON 形态（type/recipient/subject/content/metadata），任意 2xx 视为成功。
type CustomHTTPNotifier struct {
	Endpoint   string
	Headers    map[string]string
	HTTPClient *http.Client
}

// NewCustomHTTPNotifier creates a new custom HTTP notifier.
func NewCustomHTTPNotifier(endpoint string, headers map[string]string) *CustomHTTPNotifier {
	return &CustomHTTPNotifier{
		Endpoint:   endpoint,
		Headers:    headers,
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// customHTTPPayload represents the JSON payload sent to the custom endpoint.
type customHTTPPayload struct {
	Type      string                 `json:"type"`
	Recipient string                 `json:"recipient"`
	Subject   string                 `json:"subject"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func (n *CustomHTTPNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.Endpoint == "" {
		return apperrors.NewInvalidArgument("custom http notification: endpoint is required")
	}

	// SSRF 前置校验
	if err := ssrf.ValidateURL(n.Endpoint); err != nil {
		return apperrors.WrapInvalidArgumentf(err, "custom http notification: endpoint rejected")
	}

	payload := customHTTPPayload{
		Type:      notif.Type,
		Recipient: notif.Recipient,
		Subject:   notif.Subject,
		Content:   notif.Content,
		Metadata:  notif.Metadata,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "custom http notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.Endpoint, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "custom http notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	for key, value := range n.Headers {
		req.Header.Set(key, value)
	}

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "custom http notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("custom http notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
