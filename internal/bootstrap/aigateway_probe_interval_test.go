package bootstrap

import (
	"testing"
	"time"
)

// TestResolveProbeInterval covers env-var parsing contract for HealthProbeWorker.
// The worker itself already has heavy unit/integration coverage; this test
// only locks down the bootstrap-level config resolution behavior.
func TestResolveProbeInterval(t *testing.T) {
	const envKey = "AI_GATEWAY_PROBE_INTERVAL"
	const fallback = 30 * time.Second

	cases := []struct {
		name string
		env  string // os.Setenv value; "" = unset
		want time.Duration
	}{
		{name: "unset → fallback 30s", env: "", want: fallback},
		{name: "bare integer 60 → 60s", env: "60", want: 60 * time.Second},
		{name: "parseDuration '1m' → 1m", env: "1m", want: 1 * time.Minute},
		{name: "parseDuration '500ms' → 500ms", env: "500ms", want: 500 * time.Millisecond},
		{name: "parseDuration '2h' → 2h", env: "2h", want: 2 * time.Hour},
		{name: "bare integer 0 → fallback", env: "0", want: fallback},
		{name: "bare integer negative → fallback", env: "-10", want: fallback},
		{name: "whitespace → fallback", env: "   ", want: fallback},
		{name: "garbage → fallback", env: "not-a-number", want: fallback},
		{name: "negative duration str → fallback", env: "-5s", want: fallback},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envKey, tc.env)
			got := resolveProbeInterval()
			if got != tc.want {
				t.Fatalf("env=%q → got %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}
