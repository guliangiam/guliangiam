package messaging

import (
	"context"
	"crypto/rand"
	"crypto/sha256" //nolint:gosec // G401: Huawei X-WSSE spec mandates SHA256 for PasswordDigest
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// HuaweiSmsSender sends SMS via Huawei Cloud SMS API.
type HuaweiSmsSender struct {
	Endpoint   string
	AppKey     string
	AppSecret  string
	Signature  string
	TemplateID string
	Sender     string
	client     *http.Client
}

// NewHuaweiSmsSender creates a new Huawei Cloud SMS sender.
func NewHuaweiSmsSender(endpoint, appKey, appSecret, signature, templateID, sender string) *HuaweiSmsSender {
	return &HuaweiSmsSender{
		Endpoint:   endpoint,
		AppKey:     appKey,
		AppSecret:  appSecret,
		Signature:  signature,
		TemplateID: templateID,
		Sender:     sender,
		client:     ssrf.DefaultHTTPClient,
	}
}

func (s *HuaweiSmsSender) Send(ctx context.Context, phone, code string) error {
	apiURL := fmt.Sprintf("%s/sms/batchSendSms/v1", s.Endpoint)

	data := url.Values{}
	data.Set(fieldFrom, s.Sender)
	data.Set("to", toE164CN(phone))
	data.Set("templateId", s.TemplateID)
	data.Set("templateParas", fmt.Sprintf(`[%q]`, code))
	data.Set("signature", s.Signature)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "WSSE realm=\"SDP\",profile=\"UsernameToken\",type=\"Appkey\"")
	req.Header.Set("X-WSSE", buildWSSEHeader(s.AppKey, s.AppSecret))

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "huawei sms send")
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	decodeErr := json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode >= 300 {
		if decodeErr == nil {
			return apperrors.NewInternalf("huawei sms send failed: status %d: %v", resp.StatusCode, result["description"])
		}
		return apperrors.NewInternalf("huawei sms send failed: status %d", resp.StatusCode)
	}
	if decodeErr != nil {
		return apperrors.WrapInternalf(decodeErr, "huawei sms decode response")
	}
	respCode, ok := result[fieldCode].(string)
	if !ok || respCode != "000000" {
		return apperrors.NewInternalf("huawei sms send failed: code %q description %v", respCode, result["description"])
	}
	return nil
}

// buildWSSEHeader constructs the X-WSSE header for Huawei API authentication.
// PasswordDigest = Base64(SHA256(Nonce + Created + AppSecret)),
// Created is a UTC ISO8601 timestamp, Nonce is a UUID without dashes.
// https://support.huaweicloud.com/intl/zh-cn/devg-msgsms/sms_04_0012.html
func buildWSSEHeader(appKey, appSecret string) string {
	created := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	nonce := huaweiNonce()
	digest := sha256.Sum256([]byte(nonce + created + appSecret))
	passwordDigest := base64.StdEncoding.EncodeToString(digest[:])
	return fmt.Sprintf(`UsernameToken Username=%q,PasswordDigest=%q,Nonce=%q,Created=%q`,
		appKey, passwordDigest, nonce, created)
}

func huaweiNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
