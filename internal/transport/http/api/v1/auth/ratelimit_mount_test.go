package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 登录后的控制台引导会连续读取 /auth/account 与 /auth/permissions。这两个读端点
// 曾经和凭据端点共用 AuthRateLimit（10/min、burst 5），导致登录成功后几秒内就
// 收到 429，前端表现为「登录不进去」。本用例锁定拆分后的挂载关系：
//   - 读端点走 DefaultAPIRateLimit，连续请求不得 429；
//   - 凭据端点仍走严格桶，burst 用尽后必须 429。
func TestAuthRoutes_SessionReadsUseGeneralLimit(t *testing.T) {
	newRouter := func() http.Handler {
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				defer func() {
					if rec := recover(); rec != nil {
						w.WriteHeader(http.StatusInternalServerError)
					}
				}()
				next.ServeHTTP(w, req)
			})
		})
		RegisterChiRoutes(r, Handler{})
		return r
	}

	t.Run("session reads never hit the credential bucket", func(t *testing.T) {
		router := newRouter()
		for i := 0; i < 12; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/permissions", nil)
			req.RemoteAddr = "198.51.100.10:1234"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			require.NotEqual(t, http.StatusTooManyRequests, rec.Code,
				"request %d of /auth/permissions was throttled by the credential bucket", i+1)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		}
	})

	t.Run("credential endpoint still enforces its burst", func(t *testing.T) {
		router := newRouter()
		throttled := false
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			req.RemoteAddr = "198.51.100.20:1234"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code == http.StatusTooManyRequests {
				throttled = true
				break
			}
		}
		assert.True(t, throttled, "credential endpoint must throttle once the burst is exhausted")
	})
}
