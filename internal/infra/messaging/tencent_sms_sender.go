package messaging

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// toE164CN normalizes a mainland-anchored number to E.164: numbers already in
// international form are kept, bare national numbers get the +86 country code.
func toE164CN(phone string) string {
	trimmed := strings.TrimSpace(phone)
	if strings.HasPrefix(trimmed, "+") {
		return trimmed
	}
	return "+86" + trimmed
}

// TencentSmsSender sends SMS via Tencent Cloud SMS API (TC3-HMAC-SHA256 signature v3).
type TencentSmsSender struct {
	SecretID   string
	SecretKey  string
	SdkAppID   string
	SignName   string
	TemplateID string
	Region     string
	client     *http.Client
}

// tencentSMSDefaultRegion is the fallback X-TC-Region value: a required public
// header for SendSms. It does not take part in the TC3 signature (signed
// headers are content-type;host only), so the existing algorithm is unchanged.
const tencentSMSDefaultRegion = "ap-guangzhou"

// NewTencentSmsSender creates a new Tencent Cloud SMS sender; an empty region
// falls back to tencentSMSDefaultRegion.
func NewTencentSmsSender(secretID, secretKey, sdkAppID, signName, templateID, region string) *TencentSmsSender {
	if region == "" {
		region = tencentSMSDefaultRegion
	}
	return &TencentSmsSender{
		SecretID:   secretID,
		SecretKey:  secretKey,
		SdkAppID:   sdkAppID,
		SignName:   signName,
		TemplateID: templateID,
		Region:     region,
		client:     ssrf.DefaultHTTPClient,
	}
}

const (
	tencentSmsHost    = "sms.tencentcloudapi.com"
	tencentSmsService = "sms"
	tencentSmsAction  = "SendSms"
	tencentSmsVersion = "2021-01-11"
)

// tencentSmsResponse is the API 3.0 envelope: {"Response": {...}}.
type tencentSmsResponse struct {
	Response struct {
		RequestID string `json:"RequestId"`
		Error     *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
		SendStatusSet []struct {
			Code         string `json:"Code"`
			Message      string `json:"Message"`
			PhoneNumber  string `json:"PhoneNumber"`
			SmsRequestId string `json:"SmsRequestId"`
		} `json:"SendStatusSet"`
	} `json:"Response"`
}

func (s *TencentSmsSender) Send(ctx context.Context, phone, code string) error {
	params := map[string]interface{}{
		"PhoneNumberSet":   []string{toE164CN(phone)},
		"SmsSdkAppId":      s.SdkAppID,
		"SignName":         s.SignName,
		"TemplateId":       s.TemplateID,
		"TemplateParamSet": []string{code},
	}

	payload, err := json.Marshal(params)
	if err != nil {
		return apperrors.WrapInternalf(err, "tencent sms marshal payload")
	}

	timestamp := time.Now().Unix()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+tencentSmsHost+"/", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	authorization, err := tencentTC3Authorization(s.SecretID, s.SecretKey, tencentSmsService, payload, timestamp)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TC-Action", tencentSmsAction)
	req.Header.Set("X-TC-Version", tencentSmsVersion)
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-TC-Region", s.Region)
	req.Header.Set("Authorization", authorization)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "tencent sms send")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return apperrors.WrapInternalf(err, "tencent sms read response")
	}

	var parsed tencentSmsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return apperrors.WrapInternalf(err, "tencent sms decode response")
	}

	// API 3.0 returns business errors inside Response.Error (with non-2xx status).
	if parsed.Response.Error != nil {
		return apperrors.NewInternalf("tencent sms send failed: %s: %s", parsed.Response.Error.Code, parsed.Response.Error.Message)
	}
	if resp.StatusCode >= 300 {
		return apperrors.NewInternalf("tencent sms send failed: status %d", resp.StatusCode)
	}
	// Per-number delivery status; "Ok" means the request was accepted.
	for _, status := range parsed.Response.SendStatusSet {
		if status.Code != "Ok" {
			return apperrors.NewInternalf("tencent sms send failed: %s: %s", status.Code, status.Message)
		}
	}
	return nil
}

// tencentTC3Authorization implements the TC3-HMAC-SHA256 signature algorithm.
// https://cloud.tencent.com/document/api/382/55981
func tencentTC3Authorization(secretID, secretKey, service string, payload []byte, timestamp int64) (string, error) {
	date := time.Unix(timestamp, 0).UTC().Format("20060102")

	hashedPayload := sha256.Sum256(payload)
	hexPayload := hex.EncodeToString(hashedPayload[:])

	// 1. Canonical request (POST / with no query string, signed headers content-type;host).
	canonicalHeaders := "content-type:application/json\nhost:" + tencentSmsHost + "\n"
	stringToSignHeaders := "content-type;host"
	canonicalRequest := fmt.Sprintf("POST\n/\n\n%s\n%s\n%s", canonicalHeaders, stringToSignHeaders, hexPayload)

	// 2. String to sign.
	hashedCanonical := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("TC3-HMAC-SHA256\n%d\n%s/%s/tc3_request\n%s", timestamp, date, service, hex.EncodeToString(hashedCanonical[:]))

	// 3. Derive signing key and compute signature.
	secretDate := tencentHMACSHA256([]byte("TC3"+secretKey), date)
	secretService := tencentHMACSHA256(secretDate, service)
	secretSigning := tencentHMACSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(tencentHMACSHA256(secretSigning, stringToSign))

	return fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s/%s/tc3_request, SignedHeaders=%s, Signature=%s",
		secretID, date, service, stringToSignHeaders, signature), nil
}

func tencentHMACSHA256(key []byte, msg string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	return mac.Sum(nil)
}
