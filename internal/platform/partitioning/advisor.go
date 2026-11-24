package partitioning

import (
	"context"
	"log/slog"

	domainpartitioning "github.com/guliangiam/guliangiam/internal/domain/partitioning"
)

// Decision is the ADR-011 verdict for one table at the current capacity.
type Decision struct {
	Table           string
	Size            domainpartitioning.RelationSize
	ShouldPartition bool
	Reason          string
}

// Advisor evaluates declared strategies against real relation sizes. It never
// changes schema; it only reports whether the ADR-011 exit conditions are met.
type Advisor struct {
	strategy domainpartitioning.PartitioningStrategy
	sizer    domainpartitioning.RelationSizer
	log      *slog.Logger
}

func NewAdvisor(strategy domainpartitioning.PartitioningStrategy, sizer domainpartitioning.RelationSizer, log *slog.Logger) *Advisor {
	return &Advisor{strategy: strategy, sizer: sizer, log: log}
}

// Evaluate probes every declared table and returns one decision per table.
// A probe failure is reported as a decision with the failure reason; it never
// aborts startup or the maintenance tick.
func (a *Advisor) Evaluate(ctx context.Context) []Decision {
	if a == nil || a.strategy == nil || a.sizer == nil {
		return nil
	}
	decisions := make([]Decision, 0)
	for _, strategy := range a.strategy.Strategies() {
		size, err := a.sizer.Size(ctx, strategy.Table)
		if err != nil {
			if a.log != nil {
				a.log.WarnContext(ctx, "partitioning size probe failed", "table", strategy.Table, "error", err)
			}
			decisions = append(decisions, Decision{Table: strategy.Table, Reason: "size_probe_failed"})
			continue
		}
		decision := evaluateStrategy(strategy, size)
		decision.Size = size
		decisions = append(decisions, decision)
		if a.log != nil && decision.ShouldPartition {
			a.log.InfoContext(ctx, "partitioning exit conditions met", "table", strategy.Table,
				"total_bytes", size.TotalBytes, "index_bytes", size.IndexBytes)
		}
	}
	return decisions
}

// ShouldPartition reports whether one table currently satisfies ADR-011.
func (a *Advisor) ShouldPartition(ctx context.Context, table string) (bool, error) {
	if a == nil || a.strategy == nil || a.sizer == nil {
		return false, nil
	}
	strategy, ok := a.strategy.StrategyFor(table)
	if !ok {
		return false, nil
	}
	size, err := a.sizer.Size(ctx, table)
	if err != nil {
		return false, err
	}
	return evaluateStrategy(*strategy, size).ShouldPartition, nil
}

// evaluateStrategy applies the ADR-011 exit conditions in order of cheapness:
// declared/echoed evidence first, then capacity thresholds.
func evaluateStrategy(strategy domainpartitioning.TableStrategy, size domainpartitioning.RelationSize) Decision {
	decision := Decision{Table: strategy.Table}
	switch {
	case !strategy.Enabled:
		decision.Reason = "disabled"
	case strategy.EnabledAt.IsZero():
		decision.Reason = "enabled_without_enabled_at"
	case len(strategy.Evidence) == 0:
		decision.Reason = "missing_explain_evidence"
	case strategy.MinSizeBytes > 0 && size.TotalBytes < strategy.MinSizeBytes:
		decision.Reason = "below_total_size_threshold"
	case strategy.MinIndexBytes > 0 && size.IndexBytes < strategy.MinIndexBytes:
		decision.Reason = "below_index_size_threshold"
	default:
		decision.ShouldPartition = true
		decision.Reason = "adr011_exit_conditions_met"
	}
	return decision
}
