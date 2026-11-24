package messaging

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// jdcloudSMSDefaultBaseURL / region mirror the official batchSend endpoint.
const (
	jdcloudSMSDefaultBaseURL  = "https://sms.jdcloud-api.com"
	jdcloudSMSDefaultRegion   = "cn-north-1"
	jdcloudSMSServiceName     = "sms"
	jdcloudSigningAlgorithm   = "JDCLOUD2-HMAC-SHA256"
	jdcloudSigningRequestTerm = "jdcloud2_request"
)

// JDCloudSmsSender sends SMS through 京东云 (JD Cloud) 文本短信 batchSend API
// signed with the JDCLOUD2-HMAC-SHA256 gateway signature scheme.
type JDCloudSmsSender struct {
	AccessKey  string
	SecretKey  string
	SignID     string
	TemplateID string
	Region     string
	BaseURL    string
	client     *http.Client
}

// NewJDCloudSmsSender creates a JD Cloud SMS sender.
func NewJDCloudSmsSender(accessKey, secretKey, signID, templateID, region string) *JDCloudSmsSender {
	return &JDCloudSmsSender{
		AccessKey:  accessKey,
		SecretKey:  secretKey,
		SignID:     signID,
		TemplateID: templateID,
		Region:     region,
		BaseURL:    jdcloudSMSDefaultBaseURL,
		client:     ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// jdcloudNonce returns a random hex string for x-jdcloud-nonce.
func jdcloudNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", apperrors.WrapInternalf(err, "jdcloud: nonce")
	}
	return hex.EncodeToString(buf), nil
}

// jdcloudSigningKey derives the per-scope signing key:
//
//	kDate    = HMAC(SHA256, "JDCLOUD2"+secret, date)
//	kRegion  = HMAC(SHA256, kDate, region)
//	kService = HMAC(SHA256, kRegion, service)
//	kSigning = HMAC(SHA256, kService, "jdcloud2_request")
func jdcloudSigningKey(secret, date, region, service string) []byte {
	h := func(key []byte, data string) []byte {
		m := hmac.New(sha256.New, key)
		m.Write([]byte(data))
		return m.Sum(nil)
	}
	kDate := h([]byte("JDCLOUD2"+secret), date)
	kRegion := h(kDate, region)
	kService := h(kRegion, service)
	return h(kService, jdcloudSigningRequestTerm)
}

// jdcloudAuthorization computes the JDCLOUD2-HMAC-SHA256 Authorization header
// for a POST with a JSON body and the three required headers
// (host, x-jdcloud-date, x-jdcloud-nonce).
func jdcloudAuthorization(host, path, accessKey, secretKey, region, service string, body []byte, dateCompact, dateTime, nonce string) string {
	hashedPayload := sha256.Sum256(body)
	canonicalHeaders := "host:" + host + "\n" +
		"x-jdcloud-date:" + dateTime + "\n" +
		"x-jdcloud-nonce:" + nonce + "\n"
	signedHeaders := "host;x-jdcloud-date;x-jdcloud-nonce"
	canonicalRequest := strings.Join([]string{
		http.MethodPost,
		path,
		"", // no query string
		canonicalHeaders,
		signedHeaders,
		hex.EncodeToString(hashedPayload[:]),
	}, "\n")
	hashedCanonical := sha256.Sum256([]byte(canonicalRequest))
	scope := dateCompact + "/" + region + "/" + service + "/" + jdcloudSigningRequestTerm
	stringToSign := jdcloudSigningAlgorithm + "\n" + dateTime + "\n" + scope + "\n" + hex.EncodeToString(hashedCanonical[:])

	signature := hmacSHA256Hex(jdcloudSigningKey(secretKey, dateCompact, region, service), stringToSign)
	return fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		jdcloudSigningAlgorithm, accessKey, scope, signedHeaders, signature)
}

func hmacSHA256Hex(key []byte, data string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return hex.EncodeToString(m.Sum(nil))
}

// Send dispatches a verification code via batchSend.
func (s *JDCloudSmsSender) Send(ctx context.Context, phone, code string) error {
	if s.AccessKey == "" || s.SecretKey == "" || s.SignID == "" || s.TemplateID == "" {
		return apperrors.NewInvalidArgument("jdcloud accessKey, secretKey, signId and templateId are required")
	}
	region := s.Region
	if region == "" {
		region = jdcloudSMSDefaultRegion
	}
	host := strings.TrimPrefix(strings.TrimPrefix(s.BaseURL, "https://"), "http://")
	path := "/v1/regions/" + region + "/batchSend"

	payload := map[string]any{
		"templateId": s.TemplateID,
		"signId":     s.SignID,
		"phoneList":  []string{phone},
		"params":     []string{code},
	}
	body, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "jdcloud: marshal")
	}

	now := time.Now().UTC()
	dateTime := now.Format("20060102T150405Z")
	nonce, err := jdcloudNonce()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+path, jsonReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "jdcloud: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-jdcloud-date", dateTime)
	req.Header.Set("x-jdcloud-nonce", nonce)
	req.Header.Set("Authorization", jdcloudAuthorization(host, path, s.AccessKey, s.SecretKey, region, jdcloudSMSServiceName, body, now.Format("20060102"), dateTime, nonce))

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "jdcloud: request")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapInternalf(err, "jdcloud: read response")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return apperrors.NewTooManyRequests("jdcloud: rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("jdcloud: API error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var result struct {
		RequestID string `json:"requestId"`
		Result    struct {
			Status  bool   `json:"status"`
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return apperrors.WrapInternalf(err, "jdcloud: decode response")
	}
	if !result.Result.Status {
		return apperrors.NewInternalf("jdcloud: send failed (code %s, message %s)", result.Result.Code, result.Result.Message)
	}
	return nil
}

var _ SmsSender = (*JDCloudSmsSender)(nil)
