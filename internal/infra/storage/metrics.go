package storage

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const metricNamespace = "guliangiam"

var storageResolutionTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "storage_resolution_total",
		Help:      "Organization-scoped storage provider resolutions by outcome",
	},
	[]string{"result"},
)

// recordStorageResolution counts one resolution outcome: database, disabled,
// invalid, unconfigured, error.
func recordStorageResolution(result string) {
	storageResolutionTotal.WithLabelValues(result).Inc()
}
