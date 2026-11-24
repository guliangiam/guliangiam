package crypto

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const metricNamespace = "guliangiam"

var (
	// argon2Duration 记录 Argon2id 密码哈希 P99 — 非功能需求 §1.1 阈值 P99 < 100ms。
	// Histogram [20, 50, 100, 200, 500]ms 覆盖弱到强哈希参数。
	argon2Duration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "argon2_duration_seconds",
			Help:      "Argon2id password hashing latency",
			Buckets:   []float64{0.02, 0.05, 0.1, 0.2, 0.5},
		},
		[]string{"operation", "result"},
	)

	argon2Total = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "argon2_total",
			Help:      "Total Argon2id hashing operations",
		},
		[]string{"operation", "result"},
	)
)
