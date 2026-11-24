// Package slo implements the pure SLO/error-budget arithmetic used by the
// release gate (ADR-020). It deliberately contains no Prometheus client so the
// budget math is unit-testable; cmd/slo-budget owns the network query.
package slo

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// SLI is one row of contracts/registries/slo.yaml slis[].
type SLI struct {
	ID           string `yaml:"id"`
	Name         string `yaml:"name"`
	Metric       string `yaml:"metric"`
	MetricDenom  string `yaml:"metric_denom"`
	MetricErrors string `yaml:"metric_errors"`
}

// SLO is one row of contracts/registries/slo.yaml slos[].
type SLO struct {
	Surface      string  `yaml:"surface"`
	SLI          string  `yaml:"sli"`
	Availability float64 `yaml:"availability"`
	Metric       string  `yaml:"metric"`
}

// Registry is the subset of slo.yaml the release gate needs.
type Registry struct {
	WindowDays    int   `yaml:"window_days"`
	WindowMinutes int   `yaml:"window_minutes"`
	SLIs          []SLI `yaml:"slis"`
	SLOs          []SLO `yaml:"slos"`
}

// ParseRegistry decodes slo.yaml.
func ParseRegistry(data []byte) (*Registry, error) {
	var r Registry
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse slo registry: %w", err)
	}
	if len(r.SLOs) == 0 {
		return nil, errors.New("slo registry has no slos")
	}
	return &r, nil
}

// BudgetResult is the evaluated error budget for one surface.
type BudgetResult struct {
	Surface            string  `json:"surface"`
	Availability       float64 `json:"availability"`
	AllowedErrorRatio  float64 `json:"allowedErrorRatio"`
	ObservedErrorRatio float64 `json:"observedErrorRatio"`
	ConsumedRatio      float64 `json:"consumedRatio"`
	RemainingPct       float64 `json:"remainingPct"`
	Healthy            bool    `json:"healthy"`
}

// Evaluate computes the remaining error budget for an availability target and
// an observed bad-event ratio. ConsumedRatio is capped at 0 from below; a
// ratio above the allowed error ratio yields RemainingPct 0 and Healthy=false.
func Evaluate(surface string, availability, observedErrorRatio float64) BudgetResult {
	allowed := 1 - availability/100
	if allowed <= 0 {
		return BudgetResult{Surface: surface, Availability: availability, Healthy: observedErrorRatio <= 0}
	}
	consumed := observedErrorRatio / allowed
	if consumed < 0 {
		consumed = 0
	}
	remaining := (1 - consumed) * 100
	if remaining < 0 {
		remaining = 0
	}
	return BudgetResult{
		Surface:            surface,
		Availability:       availability,
		AllowedErrorRatio:  allowed,
		ObservedErrorRatio: observedErrorRatio,
		ConsumedRatio:      consumed,
		RemainingPct:       remaining,
		Healthy:            consumed <= 1,
	}
}

// ErrorMetric turns a base request-counter metric selector into a 5xx error
// selector: guliangiam_http_requests_total{surface="management"} →
// guliangiam_http_requests_total{surface="management",status=~"5.."}.
func ErrorMetric(metric string) string {
	metric = strings.TrimSpace(metric)
	if !strings.Contains(metric, "{") {
		return metric + `{status=~"5.."}`
	}
	return strings.Replace(metric, "}", `,status=~"5.."}`, 1)
}

// Query builds the two PromQL expressions for one SLO window. increase() over
// the configured window matches the registry's rolling-window semantics.
func Query(s SLO, sli SLI, windowMinutes int) (num, den string) {
	window := strconv.Itoa(windowMinutes) + "m"
	if sli.MetricErrors != "" {
		return fmt.Sprintf("sum(increase(%s[%s]))", sli.MetricErrors, window),
			fmt.Sprintf("sum(increase(%s[%s]))", sli.Metric, window)
	}
	base := s.Metric
	if base == "" {
		base = sli.Metric
	}
	return fmt.Sprintf("sum(increase(%s[%s]))", ErrorMetric(base), window),
		fmt.Sprintf("sum(increase(%s[%s]))", base, window)
}
