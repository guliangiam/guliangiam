package captcha

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ── NetEase Yidun ──────────────────────────────────────────────────────────

func TestNetEaseYidunProvider_Contract(t *testing.T) {
	var captured *http.Request
	provider := NewNetEaseYidunProvider("captcha-id-1", "secret-id-1", "secret-key-1")
	provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
		captured = req
		return stubResponse(http.StatusOK, `{"result":true,"error":0}`)
	}}}

	ok, err := provider.Verify(context.Background(), "validate-token", "203.0.113.7")
	require.NoError(t, err)
	assert.True(t, ok)

	assert.Equal(t, http.MethodPost, captured.Method)
	assert.Equal(t, "https://c.dun.163.com/api/v2/verify", captured.URL.String())
	assert.Equal(t, "application/x-www-form-urlencoded", captured.Header.Get("Content-Type"))

	form := readForm(t, captured)
	assert.Equal(t, "captcha-id-1", form.Get("captchaId"))
	assert.Equal(t, "validate-token", form.Get("validate"))
	assert.Equal(t, "secret-id-1", form.Get("secretId"))
	assert.Equal(t, "v2", form.Get("version"))
	assert.NotEmpty(t, form.Get("timestamp"))
	assert.NotEmpty(t, form.Get("nonce"))

	// signature = MD5(sorted key+value concatenation + secretKey), lowercase hex.
	keys := make([]string, 0, len(form))
	for k := range form {
		if k == "signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, k := range keys {
		builder.WriteString(k)
		builder.WriteString(form.Get(k))
	}
	builder.WriteString("secret-key-1")
	sum := md5.Sum([]byte(builder.String()))
	assert.Equal(t, hex.EncodeToString(sum[:]), form.Get("signature"))
}

func TestNetEaseYidunProvider_VerdictAndFailures(t *testing.T) {
	t.Run("result_false_is_failed_not_error", func(t *testing.T) {
		provider := NewNetEaseYidunProvider("id", "sid", "key")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusOK, `{"result":false,"error":40011}`)
		}}}
		ok, err := provider.Verify(context.Background(), "token", "")
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("missing_credentials_unavailable", func(t *testing.T) {
		provider := NewNetEaseYidunProvider("id", "", "key")
		ok, err := provider.Verify(context.Background(), "token", "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
	t.Run("http_status_unavailable", func(t *testing.T) {
		provider := NewNetEaseYidunProvider("id", "sid", "key")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusBadGateway, "")
		}}}
		ok, err := provider.Verify(context.Background(), "token", "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
}

// ── Baidu Security Verification ────────────────────────────────────────────

func TestBaiduCaptchaProvider_Contract(t *testing.T) {
	var captured *http.Request
	provider := NewBaiduCaptchaProvider("app-id-1", "private-key-1", "my-app")
	provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
		captured = req
		return stubResponse(http.StatusOK, `{"error_code":0,"result":{"verify_result":true}}`)
	}}}

	token := url.Values{
		"user_token":    {"tok-123"},
		"user_response": {"resp-456"},
		"session_id":    {"sess-789"},
		"device_id":     {"dev-0"},
	}.Encode()
	ok, err := provider.Verify(context.Background(), token, "203.0.113.7")
	require.NoError(t, err)
	assert.True(t, ok)

	assert.Equal(t, http.MethodPost, captured.Method)
	assert.Equal(t, "https://sec-captcha.baidu.com/api/v1/check", captured.URL.String())

	form := readForm(t, captured)
	assert.Equal(t, "app-id-1", form.Get("app_id"))
	assert.Equal(t, "my-app", form.Get("app_name"))
	assert.Equal(t, "203.0.113.7", form.Get("user_ip"))
	assert.Equal(t, "tok-123", form.Get("user_token"))
	assert.Equal(t, "resp-456", form.Get("user_response"))
	assert.Equal(t, "sess-789", form.Get("session_id"))
	assert.Equal(t, "dev-0", form.Get("device_id"))

	// signature = MD5(user_token + user_ip + private_key), lowercase hex.
	sum := md5.Sum([]byte("tok-123" + "203.0.113.7" + "private-key-1"))
	assert.Equal(t, hex.EncodeToString(sum[:]), form.Get("signature"))
}

func TestBaiduCaptchaProvider_VerdictAndFailures(t *testing.T) {
	t.Run("verify_result_false", func(t *testing.T) {
		provider := NewBaiduCaptchaProvider("app", "pk", "")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusOK, `{"error_code":0,"result":{"verify_result":false}}`)
		}}}
		ok, err := provider.Verify(context.Background(), "user_token=t&user_response=r&session_id=s", "1.2.3.4")
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("api_error_code_unavailable", func(t *testing.T) {
		provider := NewBaiduCaptchaProvider("app", "pk", "")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusOK, `{"error_code":40001,"message":"invalid signature"}`)
		}}}
		ok, err := provider.Verify(context.Background(), "user_token=t&user_response=r&session_id=s", "1.2.3.4")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
	t.Run("malformed_token_invalid", func(t *testing.T) {
		provider := NewBaiduCaptchaProvider("app", "pk", "")
		ok, err := provider.Verify(context.Background(), "plain-token", "1.2.3.4")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
	})
	t.Run("missing_credentials_unavailable", func(t *testing.T) {
		provider := NewBaiduCaptchaProvider("", "pk", "")
		ok, err := provider.Verify(context.Background(), "user_token=t&user_response=r&session_id=s", "1.2.3.4")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
}

// ── reCAPTCHA Enterprise ───────────────────────────────────────────────────

func TestReCAPTCHAEnterpriseProvider_Contract(t *testing.T) {
	var captured *http.Request
	provider := NewReCAPTCHAEnterpriseProvider("site-key-1", "api-key-1", "my-project", 0.7, "login")
	provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
		captured = req
		return stubResponse(http.StatusOK, `{"score":0.9,"tokenProperties":{"valid":true,"action":"login"}}`)
	}}}

	ok, err := provider.Verify(context.Background(), "widget-token", "203.0.113.7")
	require.NoError(t, err)
	assert.True(t, ok)

	assert.Equal(t, http.MethodPost, captured.Method)
	assert.Equal(t, "/v1/projects/my-project/assessments", captured.URL.Path)
	assert.Equal(t, "api-key-1", captured.URL.Query().Get("key"))
	assert.Equal(t, "application/json", captured.Header.Get("Content-Type"))

	raw, err := io.ReadAll(captured.Body)
	require.NoError(t, err)
	var payload struct {
		Event struct {
			Token          string `json:"token"`
			SiteKey        string `json:"siteKey"`
			ExpectedAction string `json:"expectedAction"`
			UserIP         string `json:"userIp"`
		} `json:"event"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.Equal(t, "widget-token", payload.Event.Token)
	assert.Equal(t, "site-key-1", payload.Event.SiteKey)
	assert.Equal(t, "login", payload.Event.ExpectedAction)
	assert.Equal(t, "203.0.113.7", payload.Event.UserIP)
}

func TestReCAPTCHAEnterpriseProvider_VerdictPolicy(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"below_threshold", `{"score":0.3,"tokenProperties":{"valid":true,"action":"login"}}`, false},
		{"invalid_token", `{"score":0.9,"tokenProperties":{"valid":false,"action":"login"}}`, false},
		{"action_mismatch", `{"score":0.9,"tokenProperties":{"valid":true,"action":"signup"}}`, false},
		{"missing_properties", `{"score":0.9}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewReCAPTCHAEnterpriseProvider("site", "api", "proj", 0.7, "login")
			provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
				return stubResponse(http.StatusOK, tc.body)
			}}}
			ok, err := provider.Verify(context.Background(), "token", "")
			require.NoError(t, err)
			assert.Equal(t, tc.want, ok)
		})
	}
	t.Run("threshold_defaults_when_out_of_range", func(t *testing.T) {
		provider := NewReCAPTCHAEnterpriseProvider("site", "api", "proj", 0, "")
		assert.Equal(t, defaultReCAPTCHAV3ScoreThreshold, provider.ScoreThreshold)
	})
	t.Run("missing_credentials_unavailable", func(t *testing.T) {
		provider := NewReCAPTCHAEnterpriseProvider("site", "api", "", 0.5, "")
		ok, err := provider.Verify(context.Background(), "token", "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
	t.Run("http_status_unavailable", func(t *testing.T) {
		provider := NewReCAPTCHAEnterpriseProvider("site", "api", "proj", 0.5, "")
		provider.client = &http.Client{Transport: stubRoundTrip{fn: func(req *http.Request) *http.Response {
			return stubResponse(http.StatusForbidden, "")
		}}}
		ok, err := provider.Verify(context.Background(), "token", "")
		assert.False(t, ok)
		requireAppErrorCode(t, err, apperrors.CodeUnavailable)
	})
}

// ── NewProviderFromRow ─────────────────────────────────────────────────────

func TestNewProviderFromRow_ConfigMapping(t *testing.T) {
	t.Run("aliyun_scene_id", func(t *testing.T) {
		p, err := NewProviderFromRow("aliyun_captcha", "ak", "sk", map[string]any{"sceneId": "scene-1"})
		require.NoError(t, err)
		aliyun, ok := p.(*AliyunCaptchaProvider)
		require.True(t, ok)
		assert.Equal(t, "scene-1", aliyun.AppKey)
		assert.Equal(t, "ak", aliyun.AccessKeyID)
		assert.Equal(t, "sk", aliyun.AccessKeySecret)
	})
	t.Run("tencent_cam_credentials", func(t *testing.T) {
		p, err := NewProviderFromRow("tencent_captcha", "123456", "app-secret", map[string]any{
			"camSecretId": "AKIDcam", "camSecretKey": "camkey",
		})
		require.NoError(t, err)
		tc, ok := p.(*TencentCaptchaProvider)
		require.True(t, ok)
		assert.Equal(t, "AKIDcam", tc.SecretID)
		assert.Equal(t, "camkey", tc.SecretKey)
		assert.Equal(t, "123456", tc.CaptchaAppID)
		assert.Equal(t, "app-secret", tc.AppSecretKey)
	})
	t.Run("yidun_secret_id", func(t *testing.T) {
		p, err := NewProviderFromRow("netease_yidun", "cap-id", "biz-secret", map[string]any{"secretId": "sid-1"})
		require.NoError(t, err)
		y, ok := p.(*NetEaseYidunProvider)
		require.True(t, ok)
		assert.Equal(t, "sid-1", y.SecretID)
	})
	t.Run("enterprise_project_and_threshold", func(t *testing.T) {
		p, err := NewProviderFromRow("recaptcha_enterprise", "site", "api", map[string]any{
			"projectId": "proj-1", "scoreThreshold": 0.8, "action": "login",
		})
		require.NoError(t, err)
		e, ok := p.(*ReCAPTCHAEnterpriseProvider)
		require.True(t, ok)
		assert.Equal(t, "proj-1", e.ProjectID)
		assert.Equal(t, 0.8, e.ScoreThreshold)
		assert.Equal(t, "login", e.ExpectedAction)
	})
	t.Run("v3_threshold_from_string", func(t *testing.T) {
		p, err := NewProviderFromRow("recaptcha_v3", "", "secret", map[string]any{"scoreThreshold": "0.6", "action": "signup"})
		require.NoError(t, err)
		v3, ok := p.(*ReCAPTCHAV3Provider)
		require.True(t, ok)
		assert.Equal(t, 0.6, v3.ScoreThreshold)
		assert.Equal(t, "signup", v3.ExpectedAction)
	})
	t.Run("default_and_unknown_rejected", func(t *testing.T) {
		_, err := NewProviderFromRow("default", "", "", nil)
		require.Error(t, err)
		_, err = NewProviderFromRow("mystery", "a", "b", nil)
		require.Error(t, err)
	})
}

func TestFactory_VerifyCaptchaWithRow_Fallbacks(t *testing.T) {
	f := NewFactory()
	f.Register("turnstile", &recaptchaStub{verdict: true})
	f.Register("default", &recaptchaStub{verdict: true})

	// Empty row credentials fall back to the environment-registered provider.
	ok, err := f.VerifyCaptchaWithRow(context.Background(), "turnstile", "", "", "token", "", nil)
	require.NoError(t, err)
	assert.True(t, ok)

	// "default" always resolves through the factory (shared image store).
	ok, err = f.VerifyCaptchaWithRow(context.Background(), "default", "a", "b", "token", "", nil)
	require.NoError(t, err)
	assert.True(t, ok)

	// Unknown type without registration surfaces the factory error.
	_, err = f.VerifyCaptchaWithRow(context.Background(), "mystery", "a", "b", "token", "", nil)
	require.Error(t, err)
}

// recaptchaStub implements Provider for factory fallback assertions.
type recaptchaStub struct{ verdict bool }

func (s *recaptchaStub) Verify(context.Context, string, string) (bool, error) { return s.verdict, nil }
