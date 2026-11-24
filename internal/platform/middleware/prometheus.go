package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/guliangiam/guliangiam/internal/platform/registry"
)

// Namespace 是所有指标的命名空间前缀（naming.yaml N14：Prometheus
// guliangiam_{域}_{对象}_{单位}）。注册名因此为 guliangiam_http_requests_total 等。
const metricNamespace = "guliangiam"

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{fieldMethod, fieldPath, fieldStatus, fieldSurface},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			// DefBuckets: [1ms, 2.5ms, 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s]
			// 13 桶覆盖 IAM 全场景（健康检查 P99<20ms → 登录 P99<500ms → token 全周期 P99<3s）
			// 与 slo-registry.md SLO 99.9% 对齐：99.9th percentile 落在 500ms~2.5s 桶
			Buckets: prometheus.DefBuckets,
		},
		[]string{fieldMethod, fieldPath, fieldSurface},
	)

	httpRequestsInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "http_requests_in_flight",
			Help:      "Current number of HTTP requests in flight",
		},
		[]string{fieldMethod},
	)

	authLoginTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "auth_login_total",
			Help:      "Total number of login attempts",
		},
		[]string{"organization", fieldMethod, fieldStatus},
	)

	authLoginErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "auth_login_errors_total",
			Help:      "Total number of login errors",
		},
		[]string{"organization", fieldMethod, "error_type"},
	)
)

// PrometheusMiddleware records HTTP request metrics for Prometheus.
func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		method := r.Method

		httpRequestsInFlight.WithLabelValues(method).Inc()
		defer httpRequestsInFlight.WithLabelValues(method).Dec()

		// Wrap the ResponseWriter to capture the status code. statusRecorder
		// forwards Flush/Hijack/Unwrap, so SSE and WebSocket handlers still see
		// the underlying writer's capabilities.
		ww := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(ww, r)

		// Get the route pattern from chi context
		routePattern := chi.RouteContext(r.Context()).RoutePattern()
		if routePattern == "" {
			routePattern = r.URL.Path
		}

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(ww.statusCode)

		// The surface label is derived from the route template, the same
		// classifier the surface contract gate holds every declaration to
		// (手册 4.1.3), so the metric cannot disagree with the contract even
		// though this middleware runs outside the surface groups.
		surface := string(registry.SurfaceFor(routePattern))

		httpRequestsTotal.WithLabelValues(method, routePattern, status, surface).Inc()
		httpRequestDuration.WithLabelValues(method, routePattern, surface).Observe(duration)
	})
}

// RecordLoginAttempt records a login attempt metric.
func RecordLoginAttempt(org, method, status string) {
	authLoginTotal.WithLabelValues(org, method, status).Inc()
}

// RecordLoginError records a login error metric.
func RecordLoginError(org, method, errorType string) {
	authLoginErrors.WithLabelValues(org, method, errorType).Inc()
}
