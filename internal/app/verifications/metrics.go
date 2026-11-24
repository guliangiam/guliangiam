package verifications

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const metricNamespace = "guliangiam"

var verificationPepperRotationHitTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "verification_pepper_rotation_hit_total",
		Help:      "Verification code digests accepted by a retired root-key generation (rotation window observability)",
	},
	[]string{"generation", "verification_type"},
)

// recordVerificationPepperRotationHit counts a digest match attributed to
// PREVIOUS or LEGACY. CURRENT is the steady state and is not counted, so the
// series stays absent (and alertable) unless a rotation is actually in flight.
// Labels stay low-cardinality: no organization id and no verification id.
func recordVerificationPepperRotationHit(generation verificationPepperGeneration, verificationType string) {
	if generation == verificationPepperCurrent {
		return
	}
	verificationPepperRotationHitTotal.WithLabelValues(generation.String(), verificationType).Inc()
}
