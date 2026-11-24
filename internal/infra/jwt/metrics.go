package jwt

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const metricNamespace = "guliangiam"

var (
	// signDuration 记录 JWT RS256 签名 P99 — 非功能需求 §1.1 阈值 P99 < 1ms。
	// 直方图 [0.2, 0.5, 1, 2, 5]ms 覆盖从 NFR 阈值到长尾。
	signDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "jwt_sign_duration_seconds",
			Help:      "JWT RS256 signing latency",
			Buckets:   []float64{0.0002, 0.0005, 0.001, 0.002, 0.005},
		},
		[]string{"token_type", "result"},
	)

	signTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "jwt_sign_total",
			Help:      "Total JWT signing operations",
		},
		[]string{"token_type", "result"},
	)
)
