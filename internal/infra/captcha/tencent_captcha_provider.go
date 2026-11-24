package captcha

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TencentCaptchaProvider verifies Tencent Cloud CAPTCHA (验证码 2.0) tickets via
// the API 3.0 DescribeCaptchaResult RPC.
//
// Official contract (verified 2026-11 against
// https://cloud.tencent.com/document/product/1110/36926):
//   - endpoint https://captcha.tencentcloudapi.com, service "captcha",
//     version 2019-07-22, action DescribeCaptchaResult, signed with
//     TC3-HMAC-SHA256 over CAM SecretId/SecretKey (same derivation chain as
//     internal/infra/messaging/email_tencent_ses.go); Region is not required.
//   - required inputs: CaptchaAppId, CaptchaType=9, Ticket, Randstr, UserIp,
//     AppSecretKey.
//   - Response.CaptchaCode == 1 means the ticket passed; any other code
//     (2/7/8/9/15/16/21/100…) is a failed verification, not a transport error.
//     Response.Error is an API-level failure and makes the provider unavailable.
//
// The web widget (TJCaptcha.js, https://cloud.tencent.com/document/product/1110/36841)
// returns {ret, ticket, randstr}; the verify token is the URL-encoded query
// string "ticket=...&randstr=..." — the same composite-token shape GeeTest v4 uses.
type TencentCaptchaProvider struct {
	// CAM API credentials used for TC3-HMAC-SHA256 request signing.
	SecretID  string
	SecretKey string
	// Captcha application credentials issued in the console.
	CaptchaAppID string
	AppSecretKey string

	Endpoint string
	client   *http.Client
}

// NewTencentCaptchaProvider creates a Tencent Cloud CAPTCHA provider.
func NewTencentCaptchaProvider(secretID, secretKey, captchaAppID, appSecretKey string) *TencentCaptchaProvider {
	return &TencentCaptchaProvider{
		SecretID:     secretID,
		SecretKey:    secretKey,
		CaptchaAppID: captchaAppID,
		AppSecretKey: appSecretKey,
		Endpoint:     "https://captcha.tencentcloudapi.com",
		client:       ssrf.DefaultHTTPClient,
	}
}

// Verify performs the server-side ticket check (DescribeCaptchaResult). The
// token must be the URL-encoded "ticket=...&randstr=..." pair produced by the
// widget; remoteIP is the mandatory UserIp input.
func (p *TencentCaptchaProvider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if p.SecretID == "" || p.SecretKey == "" || p.CaptchaAppID == "" || p.AppSecretKey == "" {
		return false, apperrors.NewUnavailable("tencent captcha: credentials not configured")
	}
	ticket, randstr, err := parseTencentCaptchaToken(token)
	if err != nil {
		return false, err
	}
	if remoteIP == "" {
		return false, apperrors.NewInvalidArgument("tencent captcha: user ip is required")
	}
	appID, err := strconv.Atoi(strings.TrimSpace(p.CaptchaAppID))
	if err != nil {
		return false, apperrors.WrapInvalidArgumentf(err, "tencent captcha: CaptchaAppId must be an integer")
	}

	endpoint := strings.TrimRight(p.Endpoint, "/")
	u, err := url.Parse(endpoint)
	if err != nil {
		return false, apperrors.WrapInvalidArgumentf(err, "tencent captcha: parse endpoint")
	}

	payload, err := json.Marshal(map[string]any{
		"CaptchaAppId": appID,
		"CaptchaType":  9,
		"Ticket":       ticket,
		"Randstr":      randstr,
		"UserIp":       remoteIP,
		"AppSecretKey": p.AppSecretKey,
	})
	if err != nil {
		return false, apperrors.WrapInternalf(err, "tencent captcha: marshal request")
	}

	timestamp := time.Now().Unix()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/", bytes.NewReader(payload))
	if err != nil {
		return false, apperrors.WrapInternalf(err, "tencent captcha: build request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TC-Action", "DescribeCaptchaResult")
	req.Header.Set("X-TC-Version", "2019-07-22")
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("Authorization", tencentCaptchaTC3Authorization(p.SecretID, p.SecretKey, payload, timestamp, u.Host))

	resp, err := p.client.Do(req)
	if err != nil {
		return false, apperrors.WrapUnavailablef(err, "tencent captcha verify")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, apperrors.NewUnavailablef("tencent captcha verify: unexpected status %d", resp.StatusCode)
	}

	var envelope struct {
		Response struct {
			CaptchaCode int    `json:"CaptchaCode"`
			CaptchaMsg  string `json:"CaptchaMsg"`
			EvilLevel   int    `json:"EvilLevel"`
			Error       *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return false, apperrors.WrapUnavailablef(err, "tencent captcha: decode response")
	}
	if apiErr := envelope.Response.Error; apiErr != nil {
		return false, apperrors.NewUnavailablef("tencent captcha API error %s: %s", apiErr.Code, apiErr.Message)
	}
	// CaptchaCode 1 = 验证通过; EvilLevel 100 = 无感验证判定恶意流量.
	if envelope.Response.CaptchaCode != 1 {
		return false, nil
	}
	return envelope.Response.EvilLevel != 100, nil
}

// parseTencentCaptchaToken extracts the ticket/randstr pair from the widget's
// URL-encoded composite token. A bare ticket is rejected: the official contract
// requires the paired Randstr for verification.
func parseTencentCaptchaToken(token string) (ticket, randstr string, err error) {
	q, perr := url.ParseQuery(token)
	if perr != nil || q.Get("ticket") == "" || q.Get("randstr") == "" {
		return "", "", apperrors.NewInvalidArgument(`tencent captcha: token must be the URL-encoded "ticket=...&randstr=..." pair`)
	}
	return q.Get("ticket"), q.Get("randstr"), nil
}

// tencentCaptchaTC3Authorization implements the documented TC3-HMAC-SHA256
// signing chain (SecretDate = HMAC("TC3"+SecretKey, Date), scope
// {date}/captcha/tc3_request) for the captcha service with the fixed
// content-type;host signed headers:
// https://cloud.tencent.com/document/api/1110/36920
func tencentCaptchaTC3Authorization(secretID, secretKey string, payload []byte, timestamp int64, host string) string {
	const service = "captcha"
	date := time.Unix(timestamp, 0).UTC().Format("20060102")

	hashedPayload := sha256.Sum256(payload)
	hexPayload := hex.EncodeToString(hashedPayload[:])

	canonicalHeaders := "content-type:application/json\nhost:" + host + "\n"
	signedHeaders := "content-type;host"
	canonicalRequest := fmt.Sprintf("POST\n/\n\n%s\n%s\n%s", canonicalHeaders, signedHeaders, hexPayload)

	hashedCanonical := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("TC3-HMAC-SHA256\n%d\n%s/%s/tc3_request\n%s", timestamp, date, service, hex.EncodeToString(hashedCanonical[:]))

	secretDate := tencentCaptchaHMAC([]byte("TC3"+secretKey), date)
	secretService := tencentCaptchaHMAC(secretDate, service)
	secretSigning := tencentCaptchaHMAC(secretService, "tc3_request")
	signature := hex.EncodeToString(tencentCaptchaHMAC(secretSigning, stringToSign))

	return fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s/%s/tc3_request, SignedHeaders=%s, Signature=%s",
		secretID, date, service, signedHeaders, signature)
}

func tencentCaptchaHMAC(key []byte, msg string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	return mac.Sum(nil)
}

var _ Provider = (*TencentCaptchaProvider)(nil)
