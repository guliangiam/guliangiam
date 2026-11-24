package casbin

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Namespace 与 platform/middleware/prometheus.go 统一。
const metricNamespace = "guliangiam"

var (
	// enforceDuration 记录 Casbin Enforce P99 — 非功能需求 §1.1 阈值 P99 < 2ms。
	// 标签：enforcer_id / model（modelText 的短哈希，避免高基数）/ result（allow/deny/error）/ cache_hit。
	enforceDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "casbin_enforce_duration_seconds",
			Help:      "Casbin Enforce evaluation latency",
			Buckets:   []float64{0.0005, 0.001, 0.002, 0.005, 0.01, 0.02},
		},
		[]string{"enforcer_id", "model", "result", "cache_hit"},
	)

	enforceTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "casbin_enforce_total",
			Help:      "Total Casbin Enforce calls",
		},
		[]string{"enforcer_id", "result"},
	)
)

// modelMetricLabel 把完整 Casbin modelText 压缩为固定长度的哈希标签。
// 直接以 modelText 作 Prometheus label 会产生高基数（每个 model 变体一条
// 时间序列，且 label 值含多行文本）；enforcerID + model 哈希已足够定位。
func modelMetricLabel(modelText string) string {
	sum := sha256.Sum256([]byte(modelText))
	return hex.EncodeToString(sum[:6])
}
