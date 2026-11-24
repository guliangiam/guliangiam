package audit

import (
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const metricNamespace = "guliangiam"

const (
	metricResultOK    = "ok"
	metricResultError = "error"
)

// Exposed series (observability.md §1.4):
//   - guliangiam_audit_chain_pending_entries (gauge, chain_status='pending')
//   - guliangiam_audit_chain_build_duration_seconds (histogram, per worker chain task:
//     anchor or full verification)
var (
	// auditChainPendingEntries mirrors records still in chain_status='pending'.
	// The worker refreshes it at startup and on every anchor cycle; a failed
	// count keeps the last good value so a database error never reads as a
	// healthy zero backlog.
	auditChainPendingEntries = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricNamespace,
		Name:      "audit_chain_pending_entries",
		Help:      "Records with chain_status='pending' awaiting chain construction",
	})

	// auditChainBuildDuration measures one worker chain task (Anchor or full
	// VerifyChainRecords pass). The only label is the outcome: organization_id
	// and chain_id would be unbounded cardinality and are forbidden by
	// naming.yaml N14.
	auditChainBuildDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "audit_chain_build_duration_seconds",
			Help:      "Audit chain worker anchor/verify duration per chain by result",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"result"},
	)
)

func init() {
	if err := registerAuditChainMetrics(prometheus.DefaultRegisterer); err != nil {
		panic(err)
	}
}

// registerAuditChainMetrics registers the collectors once. A second call against
// the same registry is a no-op: AlreadyRegisteredError means the metric is
// already exposed, which happens when tests share a registry or a process
// bootstraps more than one composition root.
func registerAuditChainMetrics(reg prometheus.Registerer) error {
	for _, collector := range []prometheus.Collector{auditChainPendingEntries, auditChainBuildDuration} {
		err := reg.Register(collector)
		if err == nil {
			continue
		}
		var already prometheus.AlreadyRegisteredError
		if errors.As(err, &already) {
			continue
		}
		return err
	}
	return nil
}

func setAuditChainPendingEntries(n int64) {
	auditChainPendingEntries.Set(float64(n))
}

func observeAuditChainWorkerDuration(started time.Time, err error) {
	result := metricResultOK
	if err != nil {
		result = metricResultError
	}
	auditChainBuildDuration.WithLabelValues(result).Observe(time.Since(started).Seconds())
}
