package messaging

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// TencentSESEmailSender sends transactional email through Tencent Cloud SES
// SendEmail (API version 2020-10-02, signature v3 / TC3-HMAC-SHA256).
// https://cloud.tencent.com/document/api/1288/51034
//
// The body is delivered as Simple.Text (base64). Vendor boundary: accounts
// without the legacy "non-template sending" allowance must use Template
// sending, which the (to, subject, body) EmailSender contract cannot express;
// that path is registered as an open boundary, not implemented here.
type TencentSESEmailSender struct {
	SecretID  string
	SecretKey string
	From      string // verified sending address (FromEmailAddress)
	Region    string // e.g. ap-guangzhou; empty falls back to ap-guangzhou
	Endpoint  string // default https://ses.tencentcloudapi.com/
	client    *http.Client
}

// NewTencentSESEmailSender creates a Tencent Cloud SES sender.
func NewTencentSESEmailSender(secretID, secretKey, from, region, endpoint string) *TencentSESEmailSender {
	if endpoint == "" {
		endpoint = "https://ses.tencentcloudapi.com/"
	}
	if region == "" {
		region = "ap-guangzhou"
	}
	return &TencentSESEmailSender{
		SecretID:  secretID,
		SecretKey: secretKey,
		From:      from,
		Region:    region,
		Endpoint:  endpoint,
		client:    ssrf.DefaultHTTPClient,
	}
}

// Send delivers one email via SES SendEmail (plain-text body).
func (s *TencentSESEmailSender) Send(ctx context.Context, to, subject, body string) error {
	if s.SecretID == "" || s.SecretKey == "" {
		return apperrors.NewInvalidArgument("tencent ses SecretId/SecretKey are required")
	}
	if s.From == "" {
		return apperrors.NewInvalidArgument("tencent ses sender address is required")
	}
	payload, err := json.Marshal(map[string]any{
		"FromEmailAddress": s.From,
		"Destination":      []string{to},
		"Subject":          subject,
		"Simple": map[string]string{
			"Text": base64.StdEncoding.EncodeToString([]byte(body)),
		},
	})
	if err != nil {
		return apperrors.WrapInternalf(err, "tencent ses marshal payload")
	}

	timestamp := time.Now().Unix()
	authorization, err := sesTC3Authorization(s.SecretID, s.SecretKey, payload, timestamp, s.Endpoint)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TC-Action", "SendEmail")
	req.Header.Set("X-TC-Version", "2020-10-02")
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-TC-Region", s.Region)
	req.Header.Set("Authorization", authorization)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "tencent ses send")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapUnavailablef(err, "tencent ses read response")
	}
	var parsed struct {
		Response struct {
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return apperrors.WrapInternalf(err, "tencent ses decode response")
	}
	if parsed.Response.Error != nil {
		return apperrors.NewInternalf("tencent ses send failed: %s: %s", parsed.Response.Error.Code, parsed.Response.Error.Message)
	}
	if resp.StatusCode >= 300 {
		return apperrors.NewInternalf("tencent ses send failed: status %d", resp.StatusCode)
	}
	return nil
}

// sesTC3Authorization is the host-parameterized variant of
// tencentTC3Authorization (which hard-codes the SMS host in its canonical
// headers). It follows the official TC3 key derivation
// (SecretDate = HMAC("TC3"+Key, Date)):
// https://cloud.tencent.com/document/api/1288/51056
func sesTC3Authorization(secretID, secretKey string, payload []byte, timestamp int64, endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "tencent ses parse endpoint")
	}
	host := u.Host
	service := "ses"
	date := time.Unix(timestamp, 0).UTC().Format("20060102")

	hashedPayload := sha256.Sum256(payload)
	hexPayload := hex.EncodeToString(hashedPayload[:])

	canonicalHeaders := "content-type:application/json\nhost:" + host + "\n"
	stringToSignHeaders := "content-type;host"
	canonicalRequest := fmt.Sprintf("POST\n/\n\n%s\n%s\n%s", canonicalHeaders, stringToSignHeaders, hexPayload)

	hashedCanonical := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("TC3-HMAC-SHA256\n%d\n%s/%s/tc3_request\n%s", timestamp, date, service, hex.EncodeToString(hashedCanonical[:]))

	secretDate := tencentHMACSHA256([]byte("TC3"+secretKey), date)
	secretService := tencentHMACSHA256(secretDate, service)
	secretSigning := tencentHMACSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(tencentHMACSHA256(secretSigning, stringToSign))

	return fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s/%s/tc3_request, SignedHeaders=%s, Signature=%s",
		secretID, date, service, stringToSignHeaders, signature), nil
}

var _ EmailSender = (*TencentSESEmailSender)(nil)
