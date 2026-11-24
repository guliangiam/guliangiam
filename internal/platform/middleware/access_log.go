package middleware

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/guliangiam/guliangiam/internal/platform/logging"
	"github.com/guliangiam/guliangiam/internal/platform/registry"
)

// probePaths 是编排系统与抓取器轮询的端点，不写成功访问日志：
// 它们以秒级频率命中，写日志只会淹没业务行且没有诊断价值。
var probePaths = map[string]struct{}{
	"/healthz": {},
	"/livez":   {},
	"/readyz":  {},
	"/metrics": {},
}

// AccessLogMiddleware 统一访问日志（《可观测性》§1.3，字段口径见规范 12.4/12.10）。
//
// 字段一律 snake_case：request_id / trace_id / span_id / duration_ms；path 优先取
// chi 路由模板而非原始 URL，避免 UUID 把日志基数打散。不记录 query 与请求体。
//
// 采样由 internal/platform/logging.Handler 统一裁决：正常 INFO 访问日志按
// trace_id 哈希 10%；慢请求（>500ms）、ERROR/WARN 100%；探活成功 0%。
// 本中间件只负责访问日志；5xx 的错误码/错误类型由 route.WriteError 集中记录，
// 两者不是同一条记录，禁止在这里重复输出错误详情。
//
// panic 分支不在此处落日志：请求在 ServeHTTP 内 panic 时会直接上抛给
// transport 层 middleware.Recoverer（生产链），由它写带 stack 的审计级日志；
// 此处若再写一行反而会得到「status 仍是 200」的错误状态。
func AccessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, probe := probePaths[r.URL.Path]; probe {
			started := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(recorder, r)

			// §12.10：探活成功 0%；失败仍要 100% 可见（4xx=WARN，5xx=ERROR）。
			if recorder.statusCode < http.StatusBadRequest {
				return
			}
			level := slog.LevelWarn
			if recorder.statusCode >= http.StatusInternalServerError {
				level = slog.LevelError
			}
			writeAccessLog(r, recorder, r.URL.Path, time.Since(started), level)
			return
		}

		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		elapsed := time.Since(started)

		routePattern := chi.RouteContext(r.Context()).RoutePattern()
		if routePattern == "" {
			routePattern = r.URL.Path
		}

		level := slog.LevelInfo
		if recorder.statusCode >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		writeAccessLog(r, recorder, routePattern, elapsed, level)
	})
}

func writeAccessLog(r *http.Request, recorder *statusRecorder, routePattern string, elapsed time.Duration, level slog.Level) {
	ids := logging.RequestIDsFrom(r.Context())
	if ids.TraceID == "" {
		ids.TraceID = ids.RequestID
	}

	attrs := make([]any, 0, 13)
	attrs = append(attrs, "request_id", ids.RequestID, "trace_id", ids.TraceID)
	if ids.SpanID != "" {
		attrs = append(attrs, "span_id", ids.SpanID)
	}
	attrs = append(attrs,
		fieldMethod, r.Method,
		fieldPath, routePattern,
		fieldSurface, string(registry.SurfaceFor(routePattern)),
		fieldStatus, recorder.statusCode,
		"duration_ms", elapsed.Milliseconds(),
		"bytes", recorder.bytes,
	)

	ctx := logging.WithAccessLog(r.Context(), elapsed)
	switch level {
	case slog.LevelError:
		slog.ErrorContext(ctx, "http access", attrs...)
	case slog.LevelWarn:
		slog.WarnContext(ctx, "http access", attrs...)
	default:
		slog.InfoContext(ctx, "http access", attrs...)
	}
}

// statusRecorder 捕获状态码与响应字节数，并把 Flush / Hijack / Unwrap
// 透传给底层 writer —— 不透传时 SSE（AI 网关流式）会因为
// `w.(http.Flusher)` 断言失败而直接 500。
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
	bytes      int64
}

func (rw *statusRecorder) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *statusRecorder) Write(p []byte) (int, error) {
	written, err := rw.ResponseWriter.Write(p)
	rw.bytes += int64(written)
	return written, err
}

// Flush 为 http.Flusher 提供透传（SSE / 长轮询）。
func (rw *statusRecorder) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack 为 http.Hijacker 提供透传（WebSocket 场景）。
func (rw *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

// Unwrap 让 http.ResponseController 能穿透本包装器拿到底层 writer。
func (rw *statusRecorder) Unwrap() http.ResponseWriter { return rw.ResponseWriter }
