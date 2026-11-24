package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

func TestRateLimit_BasicEnforcement(t *testing.T) {
	config := RateLimiterConfig{
		Rate:            1, // 1 request per second
		Burst:           2, // allow 2 burst
		CleanupInterval: 1 * time.Hour,
		MaxAge:          1 * time.Hour,
	}

	called := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	})
	handler := RateLimit(config)(next)

	// First request should pass
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.RemoteAddr = "1.2.3.4:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, 1, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimit_RateLimitHeaders(t *testing.T) {
	config := RateLimiterConfig{
		Rate:            100,
		Burst:           200,
		CleanupInterval: 1 * time.Hour,
		MaxAge:          1 * time.Hour,
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RateLimit(config)(next)

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.RemoteAddr = "1.2.3.4:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Successful request should not have rate limit headers
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestExtractIP_EdgeCases(t *testing.T) {
	t.Run("multiple_proxies", func(t *testing.T) {
		r := &http.Request{
			Header:     http.Header{"X-Forwarded-For": []string{"1.1.1.1, 2.2.2.2, 3.3.3.3"}},
			RemoteAddr: "4.4.4.4",
		}
		ip := util.ClientIP(r)
		assert.Equal(t, "3.3.3.3", ip, "should take the rightmost untrusted hop in X-Forwarded-For")
	})

	t.Run("empty_xff_falls_back_to_remoteaddr", func(t *testing.T) {
		r := &http.Request{
			Header:     http.Header{"X-Forwarded-For": []string{""}},
			RemoteAddr: "5.5.5.5:8080",
		}
		ip := util.ClientIP(r)
		assert.Equal(t, "5.5.5.5", ip)
	})
}

func TestCSRFTokenMiddleware_EdgeCases(t *testing.T) {
	t.Run("empty_cookie_and_header", func(t *testing.T) {
		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		})
		handler := CSRFTokenMiddleware()(next)

		req := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.False(t, called, "should reject POST without CSRF")
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("bearer_header_alone_is_not_exempt", func(t *testing.T) {
		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})
		handler := CSRFTokenMiddleware()(next)

		req := httptest.NewRequest(http.MethodDelete, "/test", http.NoBody)
		req.Header.Set("Authorization", "BEARER token123")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.False(t, called, "a raw bearer header must not bypass CSRF")
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestGenerateCSRFToken_Uniqueness(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := generateCSRFToken()
		assert.NoError(t, err)
		assert.Len(t, token, 64, "token should be 64 hex characters")
		assert.False(t, tokens[token], "token should be unique")
		tokens[token] = true
	}
}

func TestRateLimiterConfig_AuthRateLimit(t *testing.T) {
	// Auth rate limit should be ~10 req/min = 0.167 req/sec
	assert.True(t, AuthRateLimit.Rate > 0)
	assert.True(t, AuthRateLimit.Burst > 0)
	// Auth should be more restrictive than default
	assert.True(t, float64(AuthRateLimit.Rate) < float64(DefaultAPIRateLimit.Rate))
}

func TestRateLimiterConfig_SignupRateLimit(t *testing.T) {
	// Signup rate limit should be ~5 req/min
	assert.True(t, SignupRateLimit.Rate > 0)
	assert.True(t, SignupRateLimit.Burst > 0)
}
