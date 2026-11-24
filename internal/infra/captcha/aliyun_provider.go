package captcha

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AliyunCaptchaProvider verifies Alibaba Cloud CAPTCHA 2.0 tokens via the
// VerifyIntelligentCaptcha RPC (API version 2023-03-05). Contract verified
// 2026-11 against https://help.aliyun.com/zh/captcha/captcha2-0/use-cases/
// server-api-access: POST https://captcha.cn-shanghai.aliyuncs.com with the
// form-urlencoded CaptchaVerifyParam/SceneId body, ACS3-HMAC-SHA256 signature
// over host + x-acs-* headers, Success/Result.VerifyResult response envelope.
type AliyunCaptchaProvider struct {
	AccessKeyID     string
	AccessKeySecret string
	// AppKey carries the configured scene id (env ALIYUN_CAPTCHA_APP_KEY); it
	// is sent as the SceneId request parameter.
	AppKey   string
	Endpoint string
	client   *http.Client
}

// NewAliyunCaptchaProvider creates a new Alibaba Cloud CAPTCHA 2.0 provider.
func NewAliyunCaptchaProvider(accessKeyID, accessKeySecret, appKey string) *AliyunCaptchaProvider {
	return &AliyunCaptchaProvider{
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		AppKey:          appKey,
		Endpoint:        "https://captcha.cn-shanghai.aliyuncs.com",
		client:          ssrf.DefaultHTTPClient,
	}
}

// Verify submits the client-generated CaptchaVerifyParam to the verification
// API. A response with Success=true and Result.VerifyResult=false is a failed
// verification (false, nil); API-level failures (Success=false, HTTP errors,
// transport errors) mean the provider is unavailable.
func (p *AliyunCaptchaProvider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if token == "" {
		return false, apperrors.NewInvalidArgument("aliyun captcha: CaptchaVerifyParam is required")
	}
	if p.AccessKeyID == "" || p.AccessKeySecret == "" {
		return false, apperrors.NewUnavailable("aliyun captcha: access key not configured")
	}

	endpoint := strings.TrimRight(p.Endpoint, "/")
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return false, apperrors.WrapInvalidArgumentf(err, "aliyun captcha: parse endpoint")
	}

	form := url.Values{}
	form.Set("CaptchaVerifyParam", token)
	if p.AppKey != "" {
		form.Set("SceneId", p.AppKey)
	}
	body := form.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/", strings.NewReader(body))
	if err != nil {
		return false, apperrors.WrapInternalf(err, "aliyun captcha verify request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("x-acs-action", "VerifyIntelligentCaptcha")
	req.Header.Set("x-acs-version", "2023-03-05")
	req.Header.Set("x-acs-date", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	req.Header.Set("x-acs-signature-nonce", aliyunNonce())
	req.Header.Set("x-acs-content-sha256", aliyunHexSHA256([]byte(body)))
	req.Header.Set("Authorization", aliyunACS3Signature(p.AccessKeyID, p.AccessKeySecret, req.Header, parsed.Host, body))

	resp, err := p.client.Do(req)
	if err != nil {
		return false, apperrors.WrapUnavailablef(err, "aliyun captcha verify")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, apperrors.NewUnavailablef("aliyun captcha verify: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
		Success bool   `json:"Success"`
		Result  struct {
			VerifyResult bool   `json:"VerifyResult"`
			VerifyCode   string `json:"VerifyCode"`
		} `json:"Result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, apperrors.WrapUnavailablef(err, "aliyun captcha decode response")
	}

	if !result.Success {
		return false, apperrors.NewUnavailablef("aliyun captcha API error %s: %s", result.Code, result.Message)
	}
	return result.Result.VerifyResult, nil
}

// aliyunACS3Signature implements the documented ACS3-HMAC-SHA256 RPC signing:
// the host plus the x-acs-* headers are signed over an empty canonical query.
func aliyunACS3Signature(accessKeyID, accessKeySecret string, header http.Header, host, body string) string {
	signedNames := []string{"host", "x-acs-action", "x-acs-content-sha256", "x-acs-date", "x-acs-signature-nonce", "x-acs-version"}
	values := map[string]string{
		"host":                  host,
		"x-acs-action":          header.Get("x-acs-action"),
		"x-acs-content-sha256":  header.Get("x-acs-content-sha256"),
		"x-acs-date":            header.Get("x-acs-date"),
		"x-acs-signature-nonce": header.Get("x-acs-signature-nonce"),
		"x-acs-version":         header.Get("x-acs-version"),
	}
	sort.Strings(signedNames)

	var canonicalHeaders strings.Builder
	for _, name := range signedNames {
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.TrimSpace(values[name]))
		canonicalHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(signedNames, ";")

	canonicalRequest := strings.Join([]string{
		http.MethodPost, "/", "", canonicalHeaders.String(), signedHeaders, aliyunHexSHA256([]byte(body)),
	}, "\n")
	stringToSign := "ACS3-HMAC-SHA256\n" + aliyunHexSHA256([]byte(canonicalRequest))

	mac := hmac.New(sha256.New, []byte(accessKeySecret))
	mac.Write([]byte(stringToSign))
	return "ACS3-HMAC-SHA256 Credential=" + accessKeyID +
		",SignedHeaders=" + signedHeaders +
		",Signature=" + hex.EncodeToString(mac.Sum(nil))
}

func aliyunHexSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func aliyunNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}
