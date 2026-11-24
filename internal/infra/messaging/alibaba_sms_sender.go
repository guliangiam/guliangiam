package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AlibabaSmsSender sends SMS via Alibaba Cloud SMS API.
type AlibabaSmsSender struct {
	AccessKeyID     string
	AccessKeySecret string
	SignName        string
	TemplateCode    string
	client          *http.Client
}

// NewAlibabaSmsSender creates a new Alibaba Cloud SMS sender.
func NewAlibabaSmsSender(accessKeyID, accessKeySecret, signName, templateCode string) *AlibabaSmsSender {
	return &AlibabaSmsSender{
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		SignName:        signName,
		TemplateCode:    templateCode,
		client:          ssrf.DefaultHTTPClient,
	}
}

func (s *AlibabaSmsSender) Send(ctx context.Context, phone, code string) error {
	params := map[string]string{
		"AccessKeyId":      s.AccessKeyID,
		"Action":           "SendSms",
		"Format":           "JSON",
		"PhoneNumbers":     phone,
		"SignName":         s.SignName,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   generateNonce(),
		"SignatureVersion": "1.0",
		"TemplateCode":     s.TemplateCode,
		"TemplateParam":    fmt.Sprintf(`{"code":%q}`, code),
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          "2017-05-25",
	}

	// Sign per Alibaba Cloud RPC signature spec (POST + percent-encoded query).
	params["Signature"] = signRequest(params, s.AccessKeySecret)

	values := make(url.Values)
	for k, v := range params {
		values.Set(k, v)
	}
	apiURL := fmt.Sprintf("https://dysmsapi.aliyuncs.com/?%s", values.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "alibaba sms send")
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return apperrors.WrapInternalf(err, "alibaba sms decode response")
	}

	if resultCode, ok := result["Code"].(string); ok && resultCode != "OK" {
		return apperrors.NewInternalf("alibaba sms send failed: %s", result["Message"])
	}
	return nil
}
