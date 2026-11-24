package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/config"
)

type ETagConfig struct {
	HMACKey []byte
}

func DefaultETagConfig() ETagConfig {
	key := []byte(config.ETagHMACKey())
	if len(key) == 0 {
		key = make([]byte, 32)
		_, _ = rand.Read(key)
	}
	return ETagConfig{
		HMACKey: key,
	}
}

func ETagCacheMiddleware(cfg ETagConfig) func(http.Handler) http.Handler {
	if len(cfg.HMACKey) == 0 {
		cfg.HMACKey = make([]byte, 32)
		_, _ = rand.Read(cfg.HMACKey)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}

			if isWriteOperationPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// 认证面 GET 是会话态读（/auth/account、/auth/permissions 等），
			// 绝不能进浏览器 HTTP 缓存：任何可复用缓存都会让登出后的下一次
			// /auth/account 回放旧会话 body，表现为「登出无效、自动回到控制台」，
			// 同机换用户时还会读到上一个用户的数据。
			if isSessionScopedPath(r.URL.Path) {
				w.Header().Set("Cache-Control", "no-store")
				next.ServeHTTP(w, r)
				return
			}

			rw := &etagResponseWriter{
				ResponseWriter: w,
			}
			next.ServeHTTP(rw, r)

			if rw.statusCode == http.StatusNoContent || rw.body.Len() == 0 {
				rw.flushStatus(w)
				return
			}

			if rw.statusCode >= 200 && rw.statusCode < 300 && !isUncacheable(rw.Header()) {
				hash := sha256.Sum256(rw.body.Bytes())
				mac := hmac.New(sha256.New, cfg.HMACKey)
				mac.Write(hash[:])
				etagValue := `"` + hex.EncodeToString(mac.Sum(nil)) + `"`

				if matchETag(r.Header.Get("If-None-Match"), etagValue) {
					w.Header().Set("ETag", etagValue)
					w.Header().Set("Cache-Control", buildCacheControl())
					addVaryAuthHeaders(w.Header())
					w.WriteHeader(http.StatusNotModified)
					return
				}

				w.Header().Set("ETag", etagValue)
				w.Header().Set("Cache-Control", buildCacheControl())
				addVaryAuthHeaders(w.Header())
			}

			rw.flushStatus(w)
			if rw.body.Len() > 0 {
				_, _ = w.Write(rw.body.Bytes())
			}
		})
	}
}

// flushStatus writes the status the handler recorded. The status is held back
// until the response is complete because the middleware may still replace it
// with 304, and net/http ignores a second WriteHeader — a status sent on the way
// in cannot be revised on the way out.
type etagResponseWriter struct {
	http.ResponseWriter
	body        bytes.Buffer
	statusCode  int
	wroteHeader bool
}

func (rw *etagResponseWriter) flushStatus(w http.ResponseWriter) {
	if rw.wroteHeader {
		w.WriteHeader(rw.statusCode)
	}
}

func isWriteOperationPath(path string) bool {
	writePrefixes := []string{
		pathAuthLogin,
		pathAuthLogout,
		pathAuthSignup,
	}
	for _, prefix := range writePrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// isSessionScopedPath 标记会话态读端点：它们的响应随当前 Cookie 会话变化，
// 必须逐请求回源（no-store），不能复用 30s 的浏览器缓存。
func isSessionScopedPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/auth/")
}

// isUncacheable reports whether the handler already declared its response
// uncacheable. Such a response is exempt from the ETag path: attaching a
// validator would leave the body in the browser cache and let a later
// conditional request replay it, which is exactly what a handler marks
// `no-store` to prevent (43.4 读取面矩阵「不缓存」行，如 42.3.13 的对话流).
func isUncacheable(header http.Header) bool {
	return strings.Contains(strings.ToLower(header.Get("Cache-Control")), "no-store")
}

func matchETag(ifNoneMatch, etag string) bool {
	if ifNoneMatch == "" {
		return false
	}
	for _, v := range strings.Split(ifNoneMatch, ",") {
		v = strings.TrimSpace(v)
		if v == etag || v == "*" {
			return true
		}
	}
	return false
}

// buildCacheControl 声明「可存储但每次复用必须回源校验」。不能用 max-age 新鲜期：
// 控制台写后立即 invalidate+refetch 列表时，浏览器会在新鲜期内直接回放旧 body，
// 表现为新建/删除后列表最多 30s 不同步。no-cache 保留 ETag/304 的带宽收益，
// 同时保证读-after-写 逐请求一致。
func buildCacheControl() string {
	return "private, no-cache"
}

// addVaryAuthHeaders keys the browser cache on the credentials (BE-E-08):
// without Vary, a shared browser can replay user A's cached body for user B
// within max-age after a logout/login switch.
func addVaryAuthHeaders(h http.Header) {
	h.Add("Vary", "Cookie")
	h.Add("Vary", "Authorization")
}

func (rw *etagResponseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.statusCode = http.StatusOK
		rw.wroteHeader = true
	}
	return rw.body.Write(b)
}

func (rw *etagResponseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
	}
}

func (rw *etagResponseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}
