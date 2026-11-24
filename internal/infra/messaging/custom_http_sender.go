package messaging

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// CustomHTTPSender sends emails via a custom HTTP endpoint.
// Endpoint 来自用户配置，因此出站请求强制走 SSRF 防护通道。
type CustomHTTPSender struct {
	Endpoint string
	Method   string
	Headers  map[string]string
	client   *http.Client
}

// NewCustomHTTPSender creates a new custom HTTP email sender.
func NewCustomHTTPSender(endpoint, method string, headers map[string]string) *CustomHTTPSender {
	return &CustomHTTPSender{Endpoint: endpoint, Method: method, Headers: headers, client: ssrf.DefaultHTTPClient}
}

func (s *CustomHTTPSender) Send(ctx context.Context, to, subject, body string) error {
	// SSRF 前置校验：scheme whitelist + URL 合法性
	if err := ssrf.ValidateURL(s.Endpoint); err != nil {
		return apperrors.WrapInvalidArgumentf(err, "custom http endpoint rejected")
	}

	payload := map[string]interface{}{
		"to":         to,
		fieldSubject: subject,
		"body":       body,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "custom http marshal payload")
	}

	method := s.Method
	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequestWithContext(ctx, method, s.Endpoint, strings.NewReader(string(jsonPayload)))
	if err != nil {
		return err
	}
	for k, v := range s.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "custom http send email")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return apperrors.NewInternalf("custom http send email failed: status %d", resp.StatusCode)
	}
	return nil
}
