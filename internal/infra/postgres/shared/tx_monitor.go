package shared

import (
	"context"
	"log/slog"
	"time"
)

const slowTxThreshold = 50 * time.Millisecond

func MonitorTransaction(ctx context.Context, txStart time.Time, fn func() error) error {
	err := fn()
	duration := time.Since(txStart)

	if duration > slowTxThreshold {
		slog.WarnContext(ctx, "slow transaction detected",
			"tx_hold_duration_ms", duration.Milliseconds(),
		)
	}

	return err
}
