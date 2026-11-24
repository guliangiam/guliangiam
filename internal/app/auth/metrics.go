package auth

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// passwordHistoryFailOpenTotal is the alerting surface for the history check's
// single fail-open branch: the history store could not be read. It must not be
// treated as normal traffic. Naming mirrors password_gate_fail_open_total.
var passwordHistoryFailOpenTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "password_history_fail_open_total",
	Help: "Password changes allowed because the password-history lookup failed (infrastructure failure).",
})
