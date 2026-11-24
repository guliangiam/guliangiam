package captcha

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// stubRoundTrip 捕获请求并返回预置响应，用于断言与提供商的 HTTP 契约。
type stubRoundTrip struct {
	fn func(req *http.Request) *http.Response
}

func (s stubRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	return s.fn(req), nil
}

func stubResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func errorRoundTrip(err error) *http.Client {
	return &http.Client{Transport: failingTransport{err: err}}
}

type failingTransport struct{ err error }

func (f failingTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, f.err }

func requireAppErrorCode(t *testing.T, err error, want apperrors.Code) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %T: %v", err, err)
	assert.Equal(t, want, appErr.Code)
}

func readForm(t *testing.T, req *http.Request) url.Values {
	t.Helper()
	raw, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	form, err := url.ParseQuery(string(raw))
	require.NoError(t, err)
	return form
}

func TestReCAPTCHAV2Provider_Contract(t *testing.T) {
	var captured *http.Request
	provider := NewReCAPTCHAV2Provider("fake-secret")
	provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
		captured = req
		return stubResponse(http.StatusOK, `{"success":true,"challenge_ts":"2026-11-13T00:00:00Z","hostname":"example.com"}`)
	}}}

	ok, err := provider.Verify(context.Background(), "user-token", "203.0.113.7")
	require.NoError(t, err)
	assert.True(t, ok)

	assert.Equal(t, http.MethodPost, captured.Method)
	assert.Equal(t, "https://www.google.com/recaptcha/api/siteverify", captured.URL.String())
	assert.Equal(t, "application/x-www-form-urlencoded", captured.Header.Get("Content-Type"))
	form := readForm(t, captured)
	assert.Equal(t, "fake-secret", form.Get("secret"))
	assert.Equal(t, "user-token", form.Get("response"))
	assert.Equal(t, "203.0.113.7", form.Get("remoteip"))
}

func TestReCAPTCHAV2Provider_VerificationFailureIsNotError(t *testing.T) {
	provider := NewReCAPTCHAV2Provider("fake-secret")
	provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
		return stubResponse(http.StatusOK, `{"success":false,"error-codes":["invalid-input-response","missing-input-secret"]}`)
	}}}

	ok, err := provider.Verify(context.Background(), "bad-token", "")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestReCAPTCHAV2Provider_ProviderFailuresAreUnavailable(t *testing.T) {
	t.Run("transport", func(t *testing.T) {
		provider := NewReCAPTCHAV2Provider("fake-secret")
		provider.client = errorRoundTrip(errors.New("dial tcp: lookup failed"))
		ok, err := provider.Verify(context.Background(), "token", "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
	t.Run("http_status", func(t *testing.T) {
		provider := NewReCAPTCHAV2Provider("fake-secret")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusInternalServerError, "")
		}}}
		ok, err := provider.Verify(context.Background(), "token", "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
	t.Run("bad_json", func(t *testing.T) {
		provider := NewReCAPTCHAV2Provider("fake-secret")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusOK, "not-json")
		}}}
		ok, err := provider.Verify(context.Background(), "token", "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
}

func TestReCAPTCHAV3Provider_Contract(t *testing.T) {
	var captured *http.Request
	provider := NewReCAPTCHAV3Provider("fake-secret", 0.5, "login")
	provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
		captured = req
		return stubResponse(http.StatusOK, `{"success":true,"score":0.9,"action":"login"}`)
	}}}

	ok, err := provider.Verify(context.Background(), "v3-token", "203.0.113.7")
	require.NoError(t, err)
	assert.True(t, ok)

	assert.Equal(t, http.MethodPost, captured.Method)
	assert.Equal(t, "https://www.google.com/recaptcha/api/siteverify", captured.URL.String())
	form := readForm(t, captured)
	assert.Equal(t, "fake-secret", form.Get("secret"))
	assert.Equal(t, "v3-token", form.Get("response"))
	assert.Equal(t, "203.0.113.7", form.Get("remoteip"))
}

func TestReCAPTCHAV3Provider_ScoreAndActionRejection(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"low_score", `{"success":true,"score":0.2,"action":"login"}`},
		{"action_mismatch", `{"success":true,"score":0.9,"action":"signup"}`},
		{"not_success", `{"success":false,"error-codes":["timeout-or-duplicate"]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewReCAPTCHAV3Provider("fake-secret", 0.5, "login")
			provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
				return stubResponse(http.StatusOK, tt.body)
			}}}
			ok, err := provider.Verify(context.Background(), "token", "")
			require.NoError(t, err)
			assert.False(t, ok)
		})
	}
}

func TestReCAPTCHAV3Provider_ProviderFailureIsUnavailable(t *testing.T) {
	provider := NewReCAPTCHAV3Provider("fake-secret", 0.5, "")
	provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
		return stubResponse(http.StatusServiceUnavailable, "")
	}}}
	ok, err := provider.Verify(context.Background(), "token", "")
	assert.False(t, ok)
	requireAppErrorCode(t, err, apperrors.CodeUnavailable)
}

func TestHCaptchaProvider_Contract(t *testing.T) {
	var captured *http.Request
	provider := NewHCaptchaProvider("fake-secret")
	provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
		captured = req
		return stubResponse(http.StatusOK, `{"success":true,"challenge_ts":"2026-11-13T00:00:00Z","hostname":"example.com"}`)
	}}}

	ok, err := provider.Verify(context.Background(), "user-token", "203.0.113.7")
	require.NoError(t, err)
	assert.True(t, ok)

	assert.Equal(t, http.MethodPost, captured.Method)
	assert.Equal(t, "https://api.hcaptcha.com/siteverify", captured.URL.String())
	form := readForm(t, captured)
	assert.Equal(t, "fake-secret", form.Get("secret"))
	assert.Equal(t, "user-token", form.Get("response"))
	assert.Equal(t, "203.0.113.7", form.Get("remoteip"))
}

func TestHCaptchaProvider_FailureSemantics(t *testing.T) {
	t.Run("verification_failed", func(t *testing.T) {
		provider := NewHCaptchaProvider("fake-secret")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusOK, `{"success":false,"error-codes":["wrong-sitekey"]}`)
		}}}
		ok, err := provider.Verify(context.Background(), "token", "")
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("provider_unavailable", func(t *testing.T) {
		provider := NewHCaptchaProvider("fake-secret")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusBadGateway, "")
		}}}
		ok, err := provider.Verify(context.Background(), "token", "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
}

func TestGeeTestProvider_Contract(t *testing.T) {
	token := url.Values{
		"lot_number":     {"4dc3cfc2cdff448cad8d13107198d473"},
		"captcha_output": {"output-payload"},
		"pass_token":     {"pass-token-value"},
		"gen_time":       {"1700000000"},
	}.Encode()

	var captured *http.Request
	provider := NewGeeTestProvider("gt4-captcha-id", "gt4-private-key")
	provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
		captured = req
		return stubResponse(http.StatusOK, `{"status":"success","result":"success","reason":"","captcha_args":{}}`)
	}}}

	ok, err := provider.Verify(context.Background(), token, "203.0.113.7")
	require.NoError(t, err)
	assert.True(t, ok)

	assert.Equal(t, http.MethodPost, captured.Method)
	assert.Equal(t, "https://gcaptcha4.geetest.com/validate", captured.URL.Scheme+"://"+captured.URL.Host+captured.URL.Path)
	assert.Equal(t, "gt4-captcha-id", captured.URL.Query().Get("captcha_id"))

	form := readForm(t, captured)
	assert.Equal(t, "4dc3cfc2cdff448cad8d13107198d473", form.Get("lot_number"))
	assert.Equal(t, "output-payload", form.Get("captcha_output"))
	assert.Equal(t, "pass-token-value", form.Get("pass_token"))
	assert.Equal(t, "1700000000", form.Get("gen_time"))

	// 官方签名：hex HMAC-SHA256(lot_number, captcha_key)
	mac := hmac.New(sha256.New, []byte("gt4-private-key"))
	mac.Write([]byte("4dc3cfc2cdff448cad8d13107198d473"))
	assert.Equal(t, hex.EncodeToString(mac.Sum(nil)), form.Get("sign_token"))
}

func TestGeeTestProvider_FailureSemantics(t *testing.T) {
	t.Run("verification_failed", func(t *testing.T) {
		provider := NewGeeTestProvider("gt4-captcha-id", "gt4-private-key")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusOK, `{"status":"success","result":"fail","reason":"pass_token expire"}`)
		}}}
		ok, err := provider.Verify(context.Background(), "lot_number=L1&captcha_output=o&pass_token=p&gen_time=1", "")
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("request_error_status", func(t *testing.T) {
		provider := NewGeeTestProvider("gt4-captcha-id", "gt4-private-key")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusOK, `{"status":"error","code":"-50005","msg":"illegal gen_time"}`)
		}}}
		ok, err := provider.Verify(context.Background(), "lot_number=L1&captcha_output=o&pass_token=p&gen_time=x", "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
	t.Run("invalid_token_shape", func(t *testing.T) {
		provider := NewGeeTestProvider("gt4-captcha-id", "gt4-private-key")
		ok, err := provider.Verify(context.Background(), "plain-challenge-token", "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
	})
}

func TestAliyunCaptchaProvider_Contract(t *testing.T) {
	const verifyParam = `{"sceneId":"xV9G****","certifyId":"abc123"}`
	var captured *http.Request
	var capturedBody []byte
	provider := NewAliyunCaptchaProvider("fake-ak", "fake-sk", "scene-1")
	provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
		captured = req
		capturedBody, _ = io.ReadAll(req.Body)
		return stubResponse(http.StatusOK, `{"RequestId":"r-1","Code":"Success","Message":"success","Success":true,"Result":{"VerifyResult":true,"VerifyCode":"T001"}}`)
	}}}

	ok, err := provider.Verify(context.Background(), verifyParam, "203.0.113.7")
	require.NoError(t, err)
	assert.True(t, ok)

	assert.Equal(t, http.MethodPost, captured.Method)
	assert.Equal(t, "captcha.cn-shanghai.aliyuncs.com", captured.URL.Host)
	assert.Equal(t, "/", captured.URL.Path)
	assert.Equal(t, "application/x-www-form-urlencoded", captured.Header.Get("Content-Type"))
	form, err := url.ParseQuery(string(capturedBody))
	require.NoError(t, err)
	assert.Equal(t, verifyParam, form.Get("CaptchaVerifyParam"))
	assert.Equal(t, "scene-1", form.Get("SceneId"))

	// ACS3-HMAC-SHA256 签名头
	assert.Equal(t, "VerifyIntelligentCaptcha", captured.Header.Get("x-acs-action"))
	assert.Equal(t, "2023-03-05", captured.Header.Get("x-acs-version"))
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`, captured.Header.Get("x-acs-date"))
	assert.NotEmpty(t, captured.Header.Get("x-acs-signature-nonce"))
	sum := sha256.Sum256(capturedBody)
	assert.Equal(t, hex.EncodeToString(sum[:]), captured.Header.Get("x-acs-content-sha256"))
	assert.Regexp(t, `^ACS3-HMAC-SHA256 Credential=fake-ak,SignedHeaders=host;x-acs-action;x-acs-content-sha256;x-acs-date;x-acs-signature-nonce;x-acs-version,Signature=[0-9a-f]{64}$`,
		captured.Header.Get("Authorization"))
}

func TestAliyunCaptchaProvider_FailureSemantics(t *testing.T) {
	t.Run("verification_failed", func(t *testing.T) {
		provider := NewAliyunCaptchaProvider("fake-ak", "fake-sk", "scene-1")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusOK, `{"Success":true,"Code":"Success","Result":{"VerifyResult":false,"VerifyCode":"F001"}}`)
		}}}
		ok, err := provider.Verify(context.Background(), `{"certifyId":"abc"}`, "")
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("api_error", func(t *testing.T) {
		provider := NewAliyunCaptchaProvider("fake-ak", "fake-sk", "scene-1")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusOK, `{"Success":false,"Code":"InvalidSceneId","Message":"scene id not valid"}`)
		}}}
		ok, err := provider.Verify(context.Background(), `{"certifyId":"abc"}`, "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
	t.Run("http_status", func(t *testing.T) {
		provider := NewAliyunCaptchaProvider("fake-ak", "fake-sk", "scene-1")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusServiceUnavailable, "")
		}}}
		ok, err := provider.Verify(context.Background(), `{"certifyId":"abc"}`, "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
	t.Run("empty_token", func(t *testing.T) {
		provider := NewAliyunCaptchaProvider("fake-ak", "fake-sk", "scene-1")
		ok, err := provider.Verify(context.Background(), "", "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
	})
	t.Run("missing_credentials", func(t *testing.T) {
		provider := NewAliyunCaptchaProvider("", "", "scene-1")
		ok, err := provider.Verify(context.Background(), `{"certifyId":"abc"}`, "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
}

func TestCloudflareTurnstileProvider_Contract(t *testing.T) {
	var captured *http.Request
	provider := NewCloudflareTurnstileProvider("fake-secret")
	provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
		captured = req
		return stubResponse(http.StatusOK, `{"success":true,"challenge_ts":"2026-11-13T00:00:00Z","hostname":"example.com","action":"login"}`)
	}}}

	ok, err := provider.Verify(context.Background(), "user-token", "203.0.113.7")
	require.NoError(t, err)
	assert.True(t, ok)

	assert.Equal(t, http.MethodPost, captured.Method)
	assert.Equal(t, "https://challenges.cloudflare.com/turnstile/v0/siteverify", captured.URL.String())
	form := readForm(t, captured)
	assert.Equal(t, "fake-secret", form.Get("secret"))
	assert.Equal(t, "user-token", form.Get("response"))
	assert.Equal(t, "203.0.113.7", form.Get("remoteip"))
}

func TestCloudflareTurnstileProvider_FailureSemantics(t *testing.T) {
	t.Run("verification_failed", func(t *testing.T) {
		provider := NewCloudflareTurnstileProvider("fake-secret")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusOK, `{"success":false,"error-codes":["bad-response","timeout-or-duplicate"]}`)
		}}}
		ok, err := provider.Verify(context.Background(), "token", "")
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("provider_unavailable", func(t *testing.T) {
		provider := NewCloudflareTurnstileProvider("fake-secret")
		provider.client = errorRoundTrip(errors.New("connection refused"))
		ok, err := provider.Verify(context.Background(), "token", "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
}

func TestTencentCaptchaProvider_Contract(t *testing.T) {
	var captured *http.Request
	var capturedBody []byte
	provider := NewTencentCaptchaProvider("fake-secret-id", "fake-secret-key", "204099123", "fake-app-secret-key")
	provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
		captured = req
		raw, _ := io.ReadAll(req.Body)
		capturedBody = raw
		return stubResponse(http.StatusOK, `{"Response":{"CaptchaCode":1,"CaptchaMsg":"验证通过","EvilLevel":0,"RequestId":"req-1"}}`)
	}}}

	ok, err := provider.Verify(context.Background(), "ticket=tgt-1&randstr=%40abc", "203.0.113.7")
	require.NoError(t, err)
	assert.True(t, ok)

	assert.Equal(t, http.MethodPost, captured.Method)
	assert.Equal(t, "https://captcha.tencentcloudapi.com/", captured.URL.String())
	assert.Equal(t, "application/json", captured.Header.Get("Content-Type"))
	assert.Equal(t, "DescribeCaptchaResult", captured.Header.Get("X-TC-Action"))
	assert.Equal(t, "2019-07-22", captured.Header.Get("X-TC-Version"))

	var payload struct {
		CaptchaAppId int    `json:"CaptchaAppId"`
		CaptchaType  int    `json:"CaptchaType"`
		Ticket       string `json:"Ticket"`
		Randstr      string `json:"Randstr"`
		UserIp       string `json:"UserIp"`
		AppSecretKey string `json:"AppSecretKey"`
	}
	require.NoError(t, json.Unmarshal(capturedBody, &payload))
	assert.Equal(t, 204099123, payload.CaptchaAppId)
	assert.Equal(t, 9, payload.CaptchaType)
	assert.Equal(t, "tgt-1", payload.Ticket)
	assert.Equal(t, "@abc", payload.Randstr)
	assert.Equal(t, "203.0.113.7", payload.UserIp)
	assert.Equal(t, "fake-app-secret-key", payload.AppSecretKey)

	// 独立复算 TC3-HMAC-SHA256 签名链（官方派生从 "TC3"+SecretKey 开始，
	// scope {date}/captcha/tc3_request，signed headers 固定 content-type;host）。
	timestamp := captured.Header.Get("X-TC-Timestamp")
	require.NotEmpty(t, timestamp)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	require.NoError(t, err)
	date := time.Unix(ts, 0).UTC().Format("20060102")
	hashedPayload := sha256.Sum256(capturedBody)
	canonicalRequest := "POST\n/\n\ncontent-type:application/json\nhost:captcha.tencentcloudapi.com\n\ncontent-type;host\n" + hex.EncodeToString(hashedPayload[:])
	hashedCanonical := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := "TC3-HMAC-SHA256\n" + timestamp + "\n" + date + "/captcha/tc3_request\n" + hex.EncodeToString(hashedCanonical[:])
	kDate := tencentTestHMAC([]byte("TC3fake-secret-key"), date)
	kService := tencentTestHMAC(kDate, "captcha")
	kSigning := tencentTestHMAC(kService, "tc3_request")
	signature := hex.EncodeToString(tencentTestHMAC(kSigning, stringToSign))
	assert.Equal(t, "TC3-HMAC-SHA256 Credential=fake-secret-id/"+date+"/captcha/tc3_request, SignedHeaders=content-type;host, Signature="+signature,
		captured.Header.Get("Authorization"))
}

func tencentTestHMAC(key []byte, msg string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	return mac.Sum(nil)
}

func TestTencentCaptchaProvider_FailureSemantics(t *testing.T) {
	t.Run("captcha_code_not_1_is_failed_verification", func(t *testing.T) {
		provider := NewTencentCaptchaProvider("id", "key", "204099123", "app-key")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusOK, `{"Response":{"CaptchaCode":8,"CaptchaMsg":"verify timeout","RequestId":"req-2"}}`)
		}}}
		ok, err := provider.Verify(context.Background(), "ticket=t&randstr=r", "203.0.113.7")
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("frictionless_evil_level_rejects", func(t *testing.T) {
		provider := NewTencentCaptchaProvider("id", "key", "204099123", "app-key")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusOK, `{"Response":{"CaptchaCode":1,"EvilLevel":100,"RequestId":"req-3"}}`)
		}}}
		ok, err := provider.Verify(context.Background(), "ticket=t&randstr=r", "203.0.113.7")
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("api_error_is_unavailable", func(t *testing.T) {
		provider := NewTencentCaptchaProvider("id", "key", "204099123", "app-key")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusOK, `{"Response":{"Error":{"Code":"FailedOperation","Message":"verify failed"}}}`)
		}}}
		ok, err := provider.Verify(context.Background(), "ticket=t&randstr=r", "203.0.113.7")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
	t.Run("missing_credentials_fail_closed", func(t *testing.T) {
		provider := NewTencentCaptchaProvider("", "", "204099123", "")
		ok, err := provider.Verify(context.Background(), "ticket=t&randstr=r", "203.0.113.7")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
	t.Run("bare_ticket_rejected", func(t *testing.T) {
		provider := NewTencentCaptchaProvider("id", "key", "204099123", "app-key")
		ok, err := provider.Verify(context.Background(), "bare-ticket-without-randstr", "203.0.113.7")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
	})
	t.Run("empty_user_ip_rejected", func(t *testing.T) {
		provider := NewTencentCaptchaProvider("id", "key", "204099123", "app-key")
		ok, err := provider.Verify(context.Background(), "ticket=t&randstr=r", "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
	})
	t.Run("transport_is_unavailable", func(t *testing.T) {
		provider := NewTencentCaptchaProvider("id", "key", "204099123", "app-key")
		provider.client = errorRoundTrip(errors.New("connection refused"))
		ok, err := provider.Verify(context.Background(), "ticket=t&randstr=r", "203.0.113.7")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
}
