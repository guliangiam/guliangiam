package jwks

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const metricNamespace = "guliangiam"

var (
	// jwksRefreshTotal counts actual source refreshes (HTTP fetches), not the
	// lookups served from cache or suppressed by the unknown-kid cooldown.
	jwksRefreshTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "jwks_refresh_total",
			Help:      "Remote JWKS source refreshes by result (success/failure)",
		},
		[]string{"result"},
	)

	// jwksUnknownKidTotal counts verification-key lookups whose kid was absent
	// from the cached JWKS, by outcome (resolved/rejected/cooldown/throttled/
	// refresh_failed).
	jwksUnknownKidTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "jwks_unknown_kid_total",
			Help:      "Unknown kid lookups against the remote JWKS cache by outcome",
		},
		[]string{"outcome"},
	)
)

func recordUnknownKid(outcome string) {
	jwksUnknownKidTotal.WithLabelValues(outcome).Inc()
}
