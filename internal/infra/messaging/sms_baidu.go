package messaging

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func NewBaiduSmsSender(accessKey, secretKey, signID, templateID, invokeID string) *BaiduSmsSender {
	return &BaiduSmsSender{
		AccessKey:  accessKey,
		SecretKey:  secretKey,
		SignID:     signID,
		TemplateID: templateID,
		InvokeID:   invokeID,
		client:     ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send sends an SMS via Baidu Cloud SMS v3 API.
// Endpoint: POST https://smsv3.bj.baidubce.com/api/v3/sendSms with BCE v1 signature.
func (s *BaiduSmsSender) Send(ctx context.Context, phone, code string) error {
	const host = "smsv3.bj.baidubce.com"
	const path = "/api/v3/sendSms"
	apiURL := "https://" + host + path

	payload := map[string]any{
		"mobile":      phone,
		"template":    s.TemplateID,
		"signatureId": s.SignID,
		"contentVar":  map[string]string{fieldCode: code},
	}

	jsonData, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "baidu sms: marshal")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, jsonReader(jsonData))
	if err != nil {
		return apperrors.WrapInternalf(err, "baidu sms: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", host)
	xBceDate := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	req.Header.Set("x-bce-date", xBceDate)
	req.Header.Set("Authorization", bceAuthorizationV1(s.AccessKey, s.SecretKey, http.MethodPost, path, "", host, xBceDate))

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "baidu sms: request")
	}
	defer resp.Body.Close()

	// HTTP-level failures must surface with the status code before any
	// business-code decoding (an error body may not be valid JSON).
	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("baidu sms: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return apperrors.WrapInternalf(err, "baidu sms: decode response")
	}

	// Baidu SMS v3 returns {"code":1000,...} on success.
	if result.Code != 1000 {
		return apperrors.NewInternalf("baidu sms: API error %d: %s", result.Code, result.Message)
	}
	return nil
}

// bceAuthorizationV1 implements the BCE v1 signing algorithm.
// https://cloud.baidu.com/doc/Reference/s/9jwvz2egb
func bceAuthorizationV1(accessKey, secretKey, method, path, query, host, xBceDate string) string {
	authStringPrefix := "bce-auth-v1/" + accessKey + "/" + xBceDate + "/1800"
	signingKey := hexHMACSHA256([]byte(secretKey), authStringPrefix)
	canonicalHeaders := "host:" + uriEncodeRFC3986(host) + "\n" + "x-bce-date:" + uriEncodeRFC3986(xBceDate)
	canonicalRequest := method + "\n" + uriEncodePath(path) + "\n" + uriEncodeRFC3986(query) + "\n" + canonicalHeaders
	signature := hexHMACSHA256([]byte(signingKey), canonicalRequest)
	return authStringPrefix + "/host;x-bce-date/" + signature
}

func hexHMACSHA256(key []byte, msg string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// uriEncodeRFC3986 applies BCE's UriEncode (paramSafe): standard percent
// encoding where space is %20 (not '+') and unreserved marks (-_.~) stay literal.
func uriEncodeRFC3986(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// uriEncodePath applies BCE's UriEncode for the canonical URI: each path
// segment is encoded but '/' separators are preserved.
func uriEncodePath(s string) string {
	segments := strings.Split(s, "/")
	for i, seg := range segments {
		segments[i] = uriEncodeRFC3986(seg)
	}
	return strings.Join(segments, "/")
}

// Ensure BaiduSmsSender implements SmsSender.
var _ SmsSender = (*BaiduSmsSender)(nil)

// SubmailSmsSender sends SMS via SUBMAIL.
type SubmailSmsSender struct {
	AppID     string
	AppKey    string
	SignName  string
	ProjectID string
	client    *http.Client
}

// NewSubmailSmsSender creates a new SUBMAIL SMS sender.
