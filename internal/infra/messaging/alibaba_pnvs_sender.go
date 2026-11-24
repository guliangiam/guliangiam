package messaging

import (
	"bytes"
	"context"
	"crypto/hmac"
	cryptoRand "crypto/rand"
	"crypto/sha1" //nolint:gosec // G505: Alibaba Cloud signature v1.0 mandates HMAC-SHA1
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AlibabaPNVSSender implements phone number verification service via Alibaba Cloud PNVS.
// Unlike AlibabaSmsSender which sends SMS codes, PNVS either verifies phone numbers
// directly using a token obtained from the mobile SDK (GetMobile API) or sends a
// verification code through SendSmsVerifyCode.
type AlibabaPNVSSender struct {
	accessKeyID     string
	accessKeySecret string
	signName        string
	templateCode    string
	endpoint        string
	HTTPClient      *http.Client
}

// NewAlibabaPNVSSender creates a new Alibaba Cloud PNVS sender for GetMobile.
func NewAlibabaPNVSSender(accessKeyID, accessKeySecret string) *AlibabaPNVSSender {
	return &AlibabaPNVSSender{
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		endpoint:        "dypnsapi.aliyuncs.com",
		HTTPClient:      ssrf.DefaultHTTPClient,
	}
}

// NewAlibabaPNVSSenderWithTemplate creates a PNVS sender that can deliver SMS
// verification codes (SendSmsVerifyCode) in addition to GetMobile.
func NewAlibabaPNVSSenderWithTemplate(accessKeyID, accessKeySecret, signName, templateCode string) *AlibabaPNVSSender {
	sender := NewAlibabaPNVSSender(accessKeyID, accessKeySecret)
	sender.signName = signName
	sender.templateCode = templateCode
	return sender
}

// alibabaPNVSResponse is the API response from Alibaba PNVS GetMobile.
type alibabaPNVSResponse struct {
	Code               string `json:"Code"`
	Message            string `json:"Message"`
	RequestID          string `json:"RequestId"`
	GetMobileResultDTO struct {
		Mobile string `json:"Mobile"`
	} `json:"GetMobileResultDTO"`
}

// Send delivers a PNVS SMS verification code. The template receives the code
// through TemplateParam; PNVS keeps its own validity window.
func (s *AlibabaPNVSSender) Send(ctx context.Context, phone, code string) error {
	if s.accessKeyID == "" || s.accessKeySecret == "" {
		return apperrors.NewUnavailable("alibaba PNVS: access key not configured")
	}
	if s.signName == "" || s.templateCode == "" {
		return apperrors.NewInvalidArgument("alibaba PNVS: signName and templateCode are required")
	}

	params := map[string]string{
		"AccessKeyId":      s.accessKeyID,
		"Action":           "SendSmsVerifyCode",
		"Format":           "JSON",
		"Version":          "2017-05-25",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":   generateNonce(),
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"PhoneNumber":      strings.TrimPrefix(strings.TrimSpace(phone), "+86"),
		"SignName":         s.signName,
		"TemplateCode":     s.templateCode,
		"TemplateParam":    fmt.Sprintf(`{"code":%q}`, code),
		"ValidTime":        "600",
	}
	return s.call(ctx, params)
}

// VerifyPhone verifies a phone number using Alibaba Cloud PNVS GetMobile API.
// The token parameter is obtained from the Alibaba PNVS mobile SDK.
// Returns the verified phone number or an error.
func (s *AlibabaPNVSSender) VerifyPhone(ctx context.Context, token string) (string, error) {
	if s.accessKeyID == "" || s.accessKeySecret == "" {
		return "", apperrors.NewUnavailable("alibaba PNVS: access key not configured")
	}
	if token == "" {
		return "", apperrors.NewInvalidArgument("alibaba PNVS: verification token is required")
	}

	// Build the API request parameters
	params := map[string]string{
		"AccessKeyId":      s.accessKeyID,
		"Action":           "GetMobile",
		"Format":           "JSON",
		"Version":          "2017-05-25",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":   generateNonce(),
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"SpToken":          token,
	}

	// Sign the request per Alibaba Cloud API signature specification
	signature := signRequest(params, s.accessKeySecret)
	params["Signature"] = signature

	// Build the URL with query parameters
	values := make(url.Values)
	for k, v := range params {
		values.Set(k, v)
	}
	apiURL := fmt.Sprintf("https://%s/?%s", s.endpoint, values.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, http.NoBody)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "alibaba PNVS: create request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "alibaba PNVS: request failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "alibaba PNVS: read response")
	}

	var result alibabaPNVSResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", apperrors.WrapInternalf(err, "alibaba PNVS: parse response")
	}

	if result.Code != "OK" {
		return "", apperrors.NewInternalf("alibaba PNVS: API error %s: %s", result.Code, result.Message)
	}

	if result.GetMobileResultDTO.Mobile == "" {
		return "", apperrors.NewNotFound("alibaba PNVS: no mobile number in response")
	}

	return result.GetMobileResultDTO.Mobile, nil
}

// call signs and sends a PNVS RPC, accepting only the OK code.
func (s *AlibabaPNVSSender) call(ctx context.Context, params map[string]string) error {
	params["Signature"] = signRequest(params, s.accessKeySecret)

	values := make(url.Values)
	for key, value := range params {
		values.Set(key, value)
	}
	apiURL := fmt.Sprintf("https://%s/?%s", s.endpoint, values.Encode())

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, http.NoBody)
	if err != nil {
		return apperrors.WrapInternalf(err, "alibaba PNVS: create request")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := s.HTTPClient.Do(request)
	if err != nil {
		return apperrors.WrapInternalf(err, "alibaba PNVS: request failed")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return apperrors.WrapInternalf(err, "alibaba PNVS: read response")
	}
	var result alibabaPNVSResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return apperrors.WrapInternalf(err, "alibaba PNVS: parse response")
	}
	if result.Code != "OK" {
		return apperrors.NewInternalf("alibaba PNVS: API error %s: %s", result.Code, result.Message)
	}
	return nil
}

// signRequest computes the Alibaba Cloud API signature per their specification.
func signRequest(params map[string]string, accessKeySecret string) string {
	// Sort parameters by key
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build the canonical query string
	var buf bytes.Buffer
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(percentEncode(k))
		buf.WriteByte('=')
		buf.WriteString(percentEncode(params[k]))
	}

	// 待签名串按「HTTP 方法&路径&规范化查询串」拼接
	stringToSign := "POST&" + percentEncode("/") + "&" + percentEncode(buf.String())

	// HMAC-SHA1 signature
	mac := hmac.New(sha1.New, []byte(accessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return signature
}

// percentEncode implements Alibaba Cloud's URL encoding rules (RFC 3986 extended):
// space must be %20 (not '+'), '*' must be %2A and '~' must stay unencoded.
// Go's url.QueryEscape already emits %2A for '*' and keeps '~', so only the
// space encoding needs to be corrected.
// https://help.aliyun.com/zh/sdk/product-overview/rpc-mechanism
func percentEncode(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// generateNonce creates a random signature nonce.
func generateNonce() string {
	const charset = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 32)
	randBytes := make([]byte, 32)
	if _, err := cryptoRand.Read(randBytes); err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	for i := range b {
		b[i] = charset[int(randBytes[i])%len(charset)]
	}
	return string(b)
}

var _ SmsSender = (*AlibabaPNVSSender)(nil)
