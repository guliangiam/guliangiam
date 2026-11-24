package webauthn

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
	"github.com/guliangiam/guliangiam/internal/platform/ratelimit"
	apiauth "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	return r
}

func doRequest(t *testing.T, router chi.Router, method, path string, body io.Reader) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Result()
}

func newHandlerWithNilService() *Handler {
	return &Handler{AuthService: &appauth.Service{}}
}

func TestRealRoutes_RegistrationBegin(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	body := bytes.NewBufferString(`{"organizationSlug": "org", "username": "user"}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/auth/webauthn/registration/begin", body)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_RegistrationFinish(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	body := bytes.NewBufferString(`{"credential": {}}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/auth/webauthn/registration/finish", body)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_AuthenticationBegin(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	resp := doRequest(t, router, http.MethodPost, "/api/v1/auth/webauthn/authentication/begin", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

// finish 缺 challengeId 时必须由服务层校验拦成 400（裸错误文档），
// 而不是 panic 或 500 —— 这条断言同时锁定「handler 真的把请求体映射进命令」。
func TestRealRoutes_AuthenticationFinish_RequiresChallengeID(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	body := bytes.NewBufferString(`{"organizationSlug": "org", "username": "user", "credentialId": "abc"}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/auth/webauthn/authentication/finish", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_AuthenticationFinish(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())
	body := bytes.NewBufferString(`{"credential": {}}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/auth/webauthn/authentication/finish", body)
	assert.NotEqual(t, 0, resp.StatusCode)
}

// serveAs 发送一次带指定来源地址（和可选会话上下文）的请求。
func serveAs(t *testing.T, router chi.Router, method, path, remoteAddr string, ctx context.Context) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(`{"organizationSlug":"org","username":"user"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// WebAuthn 的注册与认证路由都必须挂认证级限流：burst 用尽后 begin/finish 都返回
// 429，且 429 响应带统一的 IETF 限流头。
func TestWebAuthnCeremonyRoutesEnforceAuthRateLimit(t *testing.T) {
	paths := []string{
		"/api/v1/auth/webauthn/registration/begin",
		"/api/v1/auth/webauthn/registration/finish",
		"/api/v1/auth/webauthn/authentication/begin",
		"/api/v1/auth/webauthn/authentication/finish",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			router := testRouter()
			RegisterChiRoutes(router, newHandlerWithNilService())

			for i := 0; i < mw.AuthRateLimit.Burst; i++ {
				rec := serveAs(t, router, http.MethodPost, path, "198.51.100.40:1234", nil)
				require.NotEqual(t, http.StatusTooManyRequests, rec.Code,
					"request %d of %s must stay inside the window", i+1, path)
			}

			rec := serveAs(t, router, http.MethodPost, path, "198.51.100.40:1234", nil)
			require.Equal(t, http.StatusTooManyRequests, rec.Code,
				"%s must return 429 once the burst is exhausted", path)
			assert.Equal(t, `10;w=60;burst=5;comment="auth-ip"`, rec.Header().Get(ratelimit.HeaderPolicy))
			assert.NotEmpty(t, rec.Header().Get(ratelimit.HeaderRetryAfter))
		})
	}
}

// 不同主体互不影响：一个 IP 的注册桶打满后，另一个 IP 和认证族仍可正常进入。
func TestWebAuthnRateLimitIsolatedPerSubjectAndFamily(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	for i := 0; i < mw.AuthRateLimit.Burst; i++ {
		rec := serveAs(t, router, http.MethodPost, "/api/v1/auth/webauthn/registration/begin", "198.51.100.50:1234", nil)
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code)
	}
	require.Equal(t, http.StatusTooManyRequests,
		serveAs(t, router, http.MethodPost, "/api/v1/auth/webauthn/registration/begin", "198.51.100.50:1234", nil).Code)

	// 另一个 IP 不受影响。
	assert.NotEqual(t, http.StatusTooManyRequests,
		serveAs(t, router, http.MethodPost, "/api/v1/auth/webauthn/registration/begin", "198.51.100.51:1234", nil).Code)
	// 同一 IP 的认证族有自己的桶。
	assert.NotEqual(t, http.StatusTooManyRequests,
		serveAs(t, router, http.MethodPost, "/api/v1/auth/webauthn/authentication/begin", "198.51.100.50:1234", nil).Code)
}

// 已登录请求按用户维度计数：换 IP 也共享同一个桶。
func TestWebAuthnRateLimitUsesUserSubjectForAuthenticatedRequests(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	sessionCtx := apiauth.WithSessionContext(context.Background(), apiauth.SessionContext{
		UserID:   "11111111-1111-1111-1111-111111111111",
		Username: "alice",
	})

	for i := 0; i < mw.AuthRateLimit.Burst; i++ {
		rec := serveAs(t, router, http.MethodPost, "/api/v1/auth/webauthn/registration/begin", "198.51.100.60:1234", sessionCtx)
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code)
	}

	// 同一用户从另一个 IP 继续：必须仍被限流。
	assert.Equal(t, http.StatusTooManyRequests,
		serveAs(t, router, http.MethodPost, "/api/v1/auth/webauthn/registration/begin", "198.51.100.61:1234", sessionCtx).Code)
	// 匿名请求（不同主体）不受该用户桶影响。
	assert.NotEqual(t, http.StatusTooManyRequests,
		serveAs(t, router, http.MethodPost, "/api/v1/auth/webauthn/registration/begin", "198.51.100.61:1234", nil).Code)
}
