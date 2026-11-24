package system

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/app/system"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterChiRoutes registers system-level routes under management surface (protected).
//
// 端点分层（Kubernetes probe 约定 + PEP fail-close）：
//
//	/api/v1/system/health       — legacy，等同 liveness（公开，PEP 白名单）
//	/api/v1/system/live         — liveness，纯进程级（公开，PEP 白名单）
//	/api/v1/system/ready        — readiness，含 DB Ping（受保护，需 read 权限）
//	/metrics                    — Prometheus scrape（受保护，system.metrics.read）
//	/api/v1/system/metrics      — JSON metrics（受保护）
//
// /healthz /livez /readyz 这三段路径由 bootstrap/chi.go 在 public / protected 组分别挂，
// 不在此 RegisterChiRoutes 内。
func RegisterChiRoutes(router chi.Router, pool *pgxpool.Pool) {
	handler := NewHandlerWithPool(pool)
	cliHandler := NewCLIHandler(system.NewCLIService())

	// ── legacy liveness（保留给老运维脚本） ──────────────────────────────────
	router.Get("/api/v1/system/health", func(w http.ResponseWriter, r *http.Request) {
		result := handler.GetLive(r.Context())
		status := http.StatusOK
		if result.Status != "ok" {
			status = http.StatusServiceUnavailable
		}
		route.WriteJSON(w, status, result)
	})

	// ── liveness probe（公开，PEP 白名单） ────────────────────────────────────
	router.Get("/api/v1/system/live", func(w http.ResponseWriter, r *http.Request) {
		result := handler.GetLive(r.Context())
		route.WriteJSON(w, http.StatusOK, result)
	})

	// ── readiness probe（受保护 — 需要 DB 可达才 ready） ────────────────────────
	router.Get("/api/v1/system/ready", func(w http.ResponseWriter, r *http.Request) {
		result := handler.GetReady(r.Context())
		status := http.StatusOK
		if result.Status != "ok" {
			status = http.StatusServiceUnavailable
		}
		route.WriteJSON(w, status, result)
	})

	router.Get("/api/v1/system/info", func(w http.ResponseWriter, r *http.Request) {
		result := handler.GetInfo(r.Context())
		route.WriteJSON(w, http.StatusOK, result)
	})

	router.Get("/api/v1/system/version", func(w http.ResponseWriter, r *http.Request) {
		result := handler.GetVersion(r.Context())
		route.WriteJSON(w, http.StatusOK, result)
	})

	// JSON metrics endpoint for API consumers.
	router.Get("/api/v1/system/metrics", func(w http.ResponseWriter, r *http.Request) {
		result := handler.GetMetrics(r.Context())
		route.WriteJSON(w, http.StatusOK, result)
	})

	router.Get("/api/v1/system/prometheus-info", func(w http.ResponseWriter, r *http.Request) {
		result := handler.GetPrometheusInfo(r.Context())
		route.WriteJSON(w, http.StatusOK, result)
	})

	router.Get("/api/v1/system/cli/{language}", cliHandler.DownloadCLI)
}
