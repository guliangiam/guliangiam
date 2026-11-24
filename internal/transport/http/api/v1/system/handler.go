package system

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Build variables injected via ldflags.
var (
	BuildVersion = "v0.0.1"
	BuildTime    = "unknown"
	CommitHash   = "unknown"
)

// VersionInfo contains system version information.
type VersionInfo struct {
	Version    string `json:"version"`
	GoVersion  string `json:"goVersion"`
	BuildTime  string `json:"buildTime"`
	CommitHash string `json:"commitHash"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
}

// HealthInfo contains system health status.
type HealthInfo struct {
	Status string `json:"status"`
}

// SystemInfo contains runtime statistics.
type SystemInfo struct {
	GoVersion    string `json:"goVersion"`
	NumGoroutine int    `json:"numGoroutine"`
	NumCPU       int    `json:"numCPU"`
	HeapAlloc    uint64 `json:"heapAlloc"`
	HeapSys      uint64 `json:"heapSys"`
	StackInuse   uint64 `json:"stackInuse"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
}

// Handler provides system-level endpoints.
type Handler struct {
	// Pool 用于 readiness probe（Ping）。nil 表示无 DB 依赖，readiness 等同 liveness。
	Pool *pgxpool.Pool
}

// NewHandler creates a new system Handler without DB pool (for liveness-only scenarios).
func NewHandler() Handler {
	return Handler{}
}

// NewHandlerWithPool creates a new Handler wired to the PostgreSQL pool for readiness checks.
func NewHandlerWithPool(pool *pgxpool.Pool) Handler {
	return Handler{Pool: pool}
}

// GetVersion returns system version information.
func (h Handler) GetVersion(_ context.Context) *VersionInfo {
	return &VersionInfo{
		Version:    BuildVersion,
		GoVersion:  runtime.Version(),
		BuildTime:  BuildTime,
		CommitHash: CommitHash,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
	}
}

// GetHealth returns system health status (legacy /api/v1/system/health).
func (h Handler) GetHealth(_ context.Context) *HealthInfo {
	return &HealthInfo{Status: "ok"}
}

// GetLive returns liveness probe result — pure process-level, 无外部依赖。
// Kubernetes liveness /healthz 应使用此端点。
func (h Handler) GetLive(_ context.Context) *HealthInfo {
	return &HealthInfo{Status: "ok"}
}

// GetReady returns readiness probe result — liveness + DB pool.Ping()。
// Kubernetes readiness /readyz 应使用此端点。
// 如果 Handler 无 DB pool，退化为 liveness（适合纯内存模式）。
func (h Handler) GetReady(ctx context.Context) *HealthInfo {
	if h.Pool == nil {
		return &HealthInfo{Status: "ok"}
	}
	if err := h.Pool.Ping(ctx); err != nil {
		return &HealthInfo{Status: fmt.Sprintf("degraded: db ping failed: %v", err)}
	}
	return &HealthInfo{Status: "ok"}
}

// GetInfo returns runtime statistics.
func (h Handler) GetInfo(_ context.Context) *SystemInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return &SystemInfo{
		GoVersion:    runtime.Version(),
		NumGoroutine: runtime.NumGoroutine(),
		NumCPU:       runtime.NumCPU(),
		HeapAlloc:    memStats.HeapAlloc,
		HeapSys:      memStats.HeapSys,
		StackInuse:   memStats.StackInuse,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
	}
}

// MetricsInfo contains Prometheus-compatible metrics in JSON format.
type MetricsInfo struct {
	GoVersion     string  `json:"goVersion"`
	NumGoroutine  int     `json:"numGoroutine"`
	NumCPU        int     `json:"numCPU"`
	HeapAllocMB   float64 `json:"heapAllocMB"`
	HeapSysMB     float64 `json:"heapSysMB"`
	StackInuseMB  float64 `json:"stackInuseMB"`
	GCPauseNs     uint64  `json:"gcPauseNs"`
	NumGC         uint32  `json:"numGC"`
	UptimeSeconds int64   `json:"uptimeSeconds"`
}

// PrometheusInfo contains Prometheus scrape configuration.
type PrometheusInfo struct {
	ScrapePath     string `json:"scrapePath"`
	ScrapeInterval string `json:"scrapeInterval"`
	MetricsFormat  string `json:"metricsFormat"`
}

var startTime = time.Now()

// GetMetrics returns Prometheus-compatible metrics in JSON format.
func (h Handler) GetMetrics(_ context.Context) *MetricsInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return &MetricsInfo{
		GoVersion:     runtime.Version(),
		NumGoroutine:  runtime.NumGoroutine(),
		NumCPU:        runtime.NumCPU(),
		HeapAllocMB:   float64(memStats.HeapAlloc) / 1024 / 1024,
		HeapSysMB:     float64(memStats.HeapSys) / 1024 / 1024,
		StackInuseMB:  float64(memStats.StackInuse) / 1024 / 1024,
		GCPauseNs:     memStats.PauseTotalNs,
		NumGC:         memStats.NumGC,
		UptimeSeconds: int64(time.Since(startTime).Seconds()),
	}
}

// GetPrometheusInfo returns Prometheus scrape configuration.
func (h Handler) GetPrometheusInfo(_ context.Context) *PrometheusInfo {
	return &PrometheusInfo{
		ScrapePath:     "/metrics",
		ScrapeInterval: "15s",
		MetricsFormat:  "prometheus/text",
	}
}
