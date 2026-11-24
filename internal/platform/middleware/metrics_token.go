// Package middleware 提供跨层可复用的 HTTP middleware。
package middleware

import (
	"net/http"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	httperr "github.com/guliangiam/guliangiam/internal/platform/http"
)

// MetricsTokenMiddleware 保护 /metrics /readyz 等实例级运维端点。
//
// 为什么不用 PEP？
//
//	PEP 是租户域隔离的 Casbin RBAC（matcher: r.org == p.org），
//	但 /metrics 是实例级端点（Prometheus 抓全实例指标，无租户概念），
//	走 PEP 会导致真实租户用户全部 403（domain mismatch）。
//
// 设计：
//   - expectedToken 为空时 fail-open 方便本地开发（但加警告头）
//   - 支持两种 header：X-Metrics-Token（自定义）或 Authorization: Bearer <token>
//   - production 必须设置 METRICS_SCRAPE_TOKEN 环境变量
func MetricsTokenMiddleware(expectedToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 开发便利：未配置 token 时 fail-open（但加警告头）
			if expectedToken == "" {
				w.Header().Set("X-Warning", "METRICS_SCRAPE_TOKEN not set — metrics exposed without auth")
				next.ServeHTTP(w, r)
				return
			}

			var presented string
			if v := r.Header.Get("X-Metrics-Token"); v != "" {
				presented = v
			} else if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				presented = strings.TrimPrefix(auth, "Bearer ")
			}

			if presented == "" {
				httperr.WriteError(w, r, apperrors.NewUnauthorized("metrics token required (X-Metrics-Token or Authorization: Bearer <token>)"))
				return
			}
			if presented != expectedToken {
				httperr.WriteError(w, r, apperrors.NewForbidden("invalid metrics token"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
