// Command slo-budget evaluates the 28-day error budget per surface against a
// live Prometheus and fails when the remaining budget is below the release
// threshold (ADR-020). It is the executable half of the release gate: CI or an
// operator runs it before promoting a release.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	slo "github.com/guliangiam/guliangiam/internal/platform/slo"
)

type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Value []any `json:"value"`
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

func querySum(ctx context.Context, client *http.Client, base, query string) (float64, error) {
	u := strings.TrimRight(base, "/") + "/api/v1/query?query=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus HTTP %d", resp.StatusCode)
	}
	var parsed promResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, err
	}
	if parsed.Status != "success" {
		return 0, fmt.Errorf("prometheus query failed: %s", parsed.Error)
	}
	total := 0.0
	for _, r := range parsed.Data.Result {
		if len(r.Value) != 2 {
			continue
		}
		s, ok := r.Value[1].(string)
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("parse prometheus value %q: %w", s, err)
		}
		total += v
	}
	return total, nil
}

func main() {
	registryPath := flag.String("registry", "contracts/registries/slo.yaml", "path to slo.yaml")
	promURL := flag.String("prometheus", "http://localhost:9090", "Prometheus base URL")
	minRemaining := flag.Float64("min-remaining", 10, "block when remaining error budget percent is below this value")
	timeout := flag.Duration("timeout", 30*time.Second, "per-query timeout")
	flag.Parse()

	data, err := os.ReadFile(*registryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slo-budget: read registry: %v\n", err)
		os.Exit(2)
	}
	reg, err := slo.ParseRegistry(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "slo-budget: %v\n", err)
		os.Exit(2)
	}
	slis := make(map[string]slo.SLI, len(reg.SLIs))
	for _, s := range reg.SLIs {
		slis[s.ID] = s
	}

	client := &http.Client{Timeout: *timeout}
	ctx := context.Background()
	results := make([]slo.BudgetResult, 0, len(reg.SLOs))
	failed := false
	for _, target := range reg.SLOs {
		sli, ok := slis[target.SLI]
		if !ok {
			fmt.Fprintf(os.Stderr, "slo-budget: surface %s references unknown SLI %s\n", target.Surface, target.SLI)
			os.Exit(2)
		}
		numExpr, denExpr := slo.Query(target, sli, reg.WindowMinutes)
		num, err := querySum(ctx, client, *promURL, numExpr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "slo-budget: surface %s numerator query: %v\n", target.Surface, err)
			os.Exit(2)
		}
		den, err := querySum(ctx, client, *promURL, denExpr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "slo-budget: surface %s denominator query: %v\n", target.Surface, err)
			os.Exit(2)
		}
		observed := 0.0
		if den > 0 {
			observed = num / den
		}
		result := slo.Evaluate(target.Surface, target.Availability, observed)
		if result.RemainingPct < *minRemaining {
			failed = true
		}
		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Surface < results[j].Surface })
	fmt.Printf("%-24s %10s %10s %12s\n", "SURFACE", "TARGET", "REMAINING", "STATUS")
	for _, r := range results {
		status := "OK"
		if r.RemainingPct < *minRemaining {
			status = "BLOCK"
		}
		fmt.Printf("%-24s %9.3f%% %9.1f%% %12s\n", r.Surface, r.Availability, r.RemainingPct, status)
	}
	if failed {
		fmt.Fprintf(os.Stderr, "slo-budget: BLOCK release — at least one surface has < %.1f%% error budget remaining\n", *minRemaining)
		os.Exit(1)
	}
}
