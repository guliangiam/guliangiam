package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// yunpianDefaultBaseURL is the 云片 SMS API host (v2, single_send.json).
const yunpianDefaultBaseURL = "https://sms.yunpian.com"

// YunpianSmsSender sends SMS through 云片 (Yunpian) v2 single_send API.
// Yunpian requires the approved signature to be embedded in the text body as
// 【签名】+ 正文, and authenticates with HTTP Basic (APIKEY as username).
type YunpianSmsSender struct {
	APIKey   string
	SignName string
	// Template is the body template; %s is replaced with the code. When empty
	// a default verification-code sentence is used.
	Template string
	BaseURL  string
	client   *http.Client
}

// NewYunpianSmsSender creates a 云片 SMS sender.
func NewYunpianSmsSender(apiKey, signName, template, baseURL string) *YunpianSmsSender {
	if baseURL == "" {
		baseURL = yunpianDefaultBaseURL
	}
	return &YunpianSmsSender{
		APIKey:   apiKey,
		SignName: signName,
		Template: template,
		BaseURL:  strings.TrimRight(baseURL, "/"),
		client:   ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// renderYunpianText composes the Yunpian text payload: 【sign】+ body.
func (s *YunpianSmsSender) renderYunpianText(code string) string {
	body := s.Template
	if body == "" {
		body = "您的验证码是%s，请勿泄露。"
	}
	if strings.Count(body, "%") >= 1 && strings.Contains(body, "%s") {
		body = fmt.Sprintf(body, code)
	} else {
		body = body + code
	}
	if s.SignName == "" {
		return body
	}
	return "【" + s.SignName + "】" + body
}

// Send submits a single SMS via 云片 v2 single_send.json.
func (s *YunpianSmsSender) Send(ctx context.Context, phone, code string) error {
	if s.APIKey == "" {
		return apperrors.NewInvalidArgument("yunpian apiKey is required")
	}

	form := url.Values{}
	form.Set("mobile", phone)
	form.Set("text", s.renderYunpianText(code))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+"/v2/sms/single_send.json", strings.NewReader(form.Encode()))
	if err != nil {
		return apperrors.WrapInternalf(err, "yunpian: create request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	req.SetBasicAuth(s.APIKey, "")

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "yunpian: request")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapInternalf(err, "yunpian: read response")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return apperrors.NewTooManyRequests("yunpian: rate limited (429)")
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		if resp.StatusCode >= 400 {
			return apperrors.NewInternalf("yunpian: API error (status %d)", resp.StatusCode)
		}
		return apperrors.WrapInternalf(err, "yunpian: decode response")
	}
	// 云片返回码 0 表示调用成功。
	if result.Code != 0 {
		return apperrors.NewInternalf("yunpian: send failed (code %d, msg %s)", result.Code, result.Msg)
	}
	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("yunpian: API error (status %d)", resp.StatusCode)
	}
	return nil
}

var _ SmsSender = (*YunpianSmsSender)(nil)
