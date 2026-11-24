package providers

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// metricNamespace 与 middleware/prometheus.go 保持一致（naming.yaml N14）。
const metricNamespace = "guliangiam"

var (
	providerResolutionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "provider_resolution_total",
			Help:      "Organization-scoped provider resolutions by outcome",
		},
		[]string{"family", "channel", "result"},
	)

	providerCacheHitTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "provider_cache_hit_total",
			Help:      "Messaging provider resolutions served from the process cache",
		},
		[]string{"channel"},
	)

	providerCacheMissTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "provider_cache_miss_total",
			Help:      "Messaging provider resolutions that had to read the database",
		},
		[]string{"channel"},
	)

	providerCacheBypassTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "provider_cache_bypass_total",
			Help:      "Messaging provider resolutions that were not cacheable for the caller's tenant scope",
		},
		[]string{"channel", "reason"},
	)

	providerCacheEvictionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "provider_cache_eviction_total",
			Help:      "Messaging provider cache entries removed before use",
		},
		[]string{"channel", "reason"},
	)

	providerConnectionTestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "provider_connection_test_total",
			Help:      "Provider connection tests by outcome",
		},
		[]string{"family", "result"},
	)
)

// recordProviderResolution counts one resolution outcome. `result` is a small
// closed vocabulary: cache, database, fallback, missing, disabled, build_error,
// error. `cache` means the process cache answered; `database` means a read
// actually reached the lookup. Labels never carry an organization ID.
func recordProviderResolution(family, channel, result string) {
	providerResolutionTotal.WithLabelValues(family, channel, result).Inc()
}

func recordProviderCacheHit(channel string) {
	providerCacheHitTotal.WithLabelValues(channel).Inc()
}

func recordProviderCacheMiss(channel string) {
	providerCacheMissTotal.WithLabelValues(channel).Inc()
}

func recordProviderCacheBypass(channel, reason string) {
	providerCacheBypassTotal.WithLabelValues(channel, reason).Inc()
}

func recordProviderCacheEviction(channel, reason string) {
	providerCacheEvictionTotal.WithLabelValues(channel, reason).Inc()
}

func recordProviderConnectionTest(family string, connected bool) {
	result := "failed"
	if connected {
		result = "connected"
	}
	providerConnectionTestTotal.WithLabelValues(family, result).Inc()
}
