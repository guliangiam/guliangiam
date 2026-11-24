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
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// VolcengineSmsSender sends SMS via Volcengine SMS API (SendSms, version 2020-01-01).
type VolcengineSmsSender struct {
	AccessKeyID     string
	AccessKeySecret string
	SmsAccount      string
	SignName        string
	TemplateID      string
	client          *http.Client
}

// NewVolcengineSmsSender creates a new Volcengine SMS sender.
func NewVolcengineSmsSender(accessKeyID, accessKeySecret, smsAccount, signName, templateID string) *VolcengineSmsSender {
	return &VolcengineSmsSender{
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		SmsAccount:      smsAccount,
		SignName:        signName,
		TemplateID:      templateID,
		client:          ssrf.DefaultHTTPClient,
	}
}

const (
	volcSmsHost    = "sms.volcengineapi.com"
	volcSmsService = "volcSMS"
	volcSmsRegion  = "cn-north-1"
	volcSmsVersion = "2020-01-01"
)

// volcSmsResponse is the TOP gateway envelope.
type volcSmsResponse struct {
	ResponseMetadata struct {
		RequestID string `json:"RequestId"`
		Error     *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	} `json:"ResponseMetadata"`
	Result struct {
		MessageID []string `json:"MessageID"`
		RequestID string   `json:"RequestId"`
	} `json:"Result"`
}

func (s *VolcengineSmsSender) Send(ctx context.Context, phone, code string) error {
	payload := map[string]interface{}{
		"SmsAccount":    s.SmsAccount,
		"Sign":          s.SignName,
		"TemplateID":    s.TemplateID,
		"TemplateParam": fmt.Sprintf(`{"code":%q}`, code),
		// Official docs: PhoneNumbers is a comma-separated string, not an array.
		"PhoneNumbers": phone,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "volcengine sms marshal payload")
	}

	query := url.Values{}
	query.Set("Action", "SendSms")
	query.Set("Version", volcSmsVersion)

	apiURL := fmt.Sprintf("https://%s/?%s", volcSmsHost, query.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	xDate := time.Now().UTC().Format("20060102T150405Z")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Date", xDate)

	authorization, err := volcSign(req, body, s.AccessKeyID, s.AccessKeySecret, volcSmsRegion, volcSmsService, xDate)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", authorization)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "volcengine sms send")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return apperrors.WrapInternalf(err, "volcengine sms read response")
	}

	var parsed volcSmsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return apperrors.WrapInternalf(err, "volcengine sms decode response")
	}

	// Business errors are returned with HTTP 200 inside ResponseMetadata.Error.
	if parsed.ResponseMetadata.Error != nil {
		return apperrors.NewInternalf("volcengine sms send failed: %s: %s", parsed.ResponseMetadata.Error.Code, parsed.ResponseMetadata.Error.Message)
	}
	if resp.StatusCode >= 300 {
		return apperrors.NewInternalf("volcengine sms send failed: status %d", resp.StatusCode)
	}
	return nil
}

// volcSign implements the Volcengine API gateway HMAC-SHA256 (V4) signature.
// https://www.volcengine.com/docs/6361/1205061 (per volc-sdk-golang base/sign.go)
func volcSign(req *http.Request, body []byte, accessKeyID, secretAccessKey, region, service, xDate string) (string, error) {
	const (
		contentSHA256Header = "X-Content-Sha256"
		dateHeader          = "X-Date"
		timeFormatV4        = "20060102T150405Z"
		signAlgorithm       = "HMAC-SHA256"
	)

	_, err := time.Parse(timeFormatV4, xDate)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "volc sign: invalid x-date")
	}
	authDate := xDate[:8]

	bodyHash := sha256.Sum256(body)
	hexBodyHash := hex.EncodeToString(bodyHash[:])
	req.Header.Set(contentSHA256Header, hexBodyHash)

	// Canonical query string: sorted keys, '+' re-encoded as %20.
	query := req.URL.Query()
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var queryBuf strings.Builder
	for i, k := range keys {
		if i > 0 {
			queryBuf.WriteString("&")
		}
		queryBuf.WriteString(k)
		queryBuf.WriteString("=")
		queryBuf.WriteString(strings.ReplaceAll(query.Get(k), "+", "%20"))
	}

	// Canonical headers: content-type, host, x-content-sha256, x-date (lowercase, sorted).
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	headers := map[string]string{
		"content-type":     req.Header.Get("Content-Type"),
		"host":             host,
		"x-content-sha256": hexBodyHash,
		"x-date":           xDate,
	}
	signed := []string{"content-type", "host", "x-content-sha256", "x-date"}
	sort.Strings(signed)

	var headerBuf strings.Builder
	for _, k := range signed {
		headerBuf.WriteString(fmt.Sprintf("%s:%s\n", k, strings.TrimSpace(headers[k])))
	}

	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.Path,
		queryBuf.String(),
		headerBuf.String(),
		strings.Join(signed, ";"),
		hexBodyHash,
	}, "\n")
	hashedCanonical := sha256.Sum256([]byte(canonicalRequest))

	credentialScope := strings.Join([]string{authDate, region, service, "request"}, "/")
	stringToSign := strings.Join([]string{
		signAlgorithm,
		xDate,
		credentialScope,
		hex.EncodeToString(hashedCanonical[:]),
	}, "\n")

	kDate := volcHMAC([]byte(secretAccessKey), authDate)
	kRegion := volcHMAC(kDate, region)
	kService := volcHMAC(kRegion, service)
	kSigning := volcHMAC(kService, "request")
	signature := hex.EncodeToString(volcHMAC(kSigning, stringToSign))

	return fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		signAlgorithm, accessKeyID, credentialScope, strings.Join(signed, ";"), signature), nil
}

func volcHMAC(key []byte, msg string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	return mac.Sum(nil)
}
