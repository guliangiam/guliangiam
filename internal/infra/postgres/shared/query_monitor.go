package shared

import (
	"context"
	"log/slog"
	"regexp"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const slowQueryThreshold = 200 * time.Millisecond

// labelQueryFingerprint 是慢查询/查询总量指标的 fingerprint 标签名。
const labelQueryFingerprint = "query_fingerprint"

var slowQueryTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "guliangiam_db_slow_query_total",
		Help: "Total number of slow database queries exceeding the threshold",
	},
	[]string{labelQueryFingerprint},
)

// queryTotal tracks the total number of queries executed, labeled by fingerprint.
// Useful for identifying N+1 query patterns and high-frequency query hot spots.
var queryTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "guliangiam_db_query_total",
		Help: "Total number of database queries executed, labeled by query fingerprint",
	},
	[]string{labelQueryFingerprint},
)

// queryDuration tracks query execution duration in seconds.
var queryDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "guliangiam_db_query_duration_seconds",
		Help:    "Database query execution duration in seconds, labeled by query fingerprint",
		Buckets: prometheus.DefBuckets,
	},
	[]string{labelQueryFingerprint},
)

var paramPlaceholderRe = regexp.MustCompile(`\$\d+`)

func fingerprint(query string) string {
	return paramPlaceholderRe.ReplaceAllString(query, "$N")
}

func MonitorQuery(ctx context.Context, query string, fn func() error) error {
	fp := fingerprint(query)
	start := time.Now()
	err := fn()
	duration := time.Since(start)

	// Increment total query counter for every query (debug-level observability).
	queryTotal.WithLabelValues(fp).Inc()
	queryDuration.WithLabelValues(fp).Observe(duration.Seconds())

	if duration > slowQueryThreshold {
		slog.WarnContext(ctx, "slow query detected",
			labelQueryFingerprint, fp,
			"duration_ms", duration.Milliseconds(),
		)
		slowQueryTotal.WithLabelValues(fp).Inc()
	} else {
		// Log at debug level for query tracing (disabled by default).
		slog.DebugContext(ctx, "db query",
			labelQueryFingerprint, fp,
			"duration_ms", duration.Milliseconds(),
		)
	}

	return err
}
