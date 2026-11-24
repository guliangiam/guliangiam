package slo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluate_WithinBudget(t *testing.T) {
	r := Evaluate("management", 99.5, 0.001) // allowed 0.005, consumed 20%
	assert.True(t, r.Healthy)
	assert.InDelta(t, 80.0, r.RemainingPct, 0.001)
}

func TestEvaluate_BudgetExhausted(t *testing.T) {
	r := Evaluate("authentication", 99.9, 0.002) // allowed 0.001
	assert.False(t, r.Healthy)
	assert.Zero(t, r.RemainingPct)
}

func TestErrorMetric(t *testing.T) {
	assert.Equal(t,
		`guliangiam_http_requests_total{surface="management",status=~"5.."}`,
		ErrorMetric(`guliangiam_http_requests_total{surface="management"}`))
	assert.Equal(t,
		`guliangiam_auth_login_total{status=~"5.."}`,
		ErrorMetric("guliangiam_auth_login_total"))
}

func TestParseRegistry(t *testing.T) {
	r, err := ParseRegistry([]byte(`
window_days: 28
window_minutes: 40320
slis:
  - id: SLI-1
    metric: guliangiam_http_requests_total{surface="management"}
slos:
  - surface: management
    sli: SLI-1
    availability: 99.5
    metric: guliangiam_http_requests_total{surface="management"}
`))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(r.SLOs))
	assert.Equal(t, 40320, r.WindowMinutes)
	num, den := Query(r.SLOs[0], r.SLIs[0], r.WindowMinutes)
	assert.Contains(t, num, "status=~")
	assert.Contains(t, den, "surface=\"management\"")
}
