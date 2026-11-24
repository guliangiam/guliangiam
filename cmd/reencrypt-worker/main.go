// Command reencrypt-worker runs the field-encryption KEK rotation online.
//
// It is the continuous variant of `migrate reencrypt`: each tick rotates at
// most -max-rows rows across the registered columns, checkpoints its cursor to
// -checkpoint, and sleeps for -interval. Operators enable it explicitly (the
// binary must be deployed; nothing starts it implicitly). Use -once for a
// single bounded pass (used by L6 verification and one-shot jobs).
//
// Usage:
//
//	reencrypt-worker -dsn "$POSTGRES_DSN" -once
//	reencrypt-worker -dsn "$POSTGRES_DSN" -interval 5m -max-rows 5000
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	crypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("reencrypt-worker", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "PostgreSQL DSN (defaults to DB_DSN/POSTGRES_DSN/DATABASE_URL)")
	interval := fs.Duration("interval", 5*time.Minute, "pause between rotation ticks (min 30s)")
	once := fs.Bool("once", false, "run one bounded tick and exit (no loop)")
	maxRows := fs.Int("max-rows", 5000, "maximum rows scanned per tick (0 = unlimited)")
	batch := fs.Int("batch", 500, "rows fetched per SQL batch")
	checkpoint := fs.String("checkpoint", defaultCheckpointPath(), "cursor checkpoint file (empty disables resume)")
	includeSealed := fs.Bool("include-sealed", true, "also re-seal registered v1 sealed payloads")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *interval < 30*time.Second && !*once {
		fmt.Fprintln(os.Stderr, "reencrypt-worker: -interval must be >= 30s")
		return 2
	}
	if *batch <= 0 || *batch > 5000 {
		fmt.Fprintln(os.Stderr, "reencrypt-worker: -batch must be 1..5000")
		return 2
	}
	if *maxRows < 0 {
		fmt.Fprintln(os.Stderr, "reencrypt-worker: -max-rows must be >= 0")
		return 2
	}
	if *dsn == "" {
		*dsn = firstNonEmpty(os.Getenv("DB_DSN"), os.Getenv("POSTGRES_DSN"), os.Getenv("DATABASE_URL"))
	}
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "reencrypt-worker: database DSN is required (-dsn or DB_DSN/POSTGRES_DSN/DATABASE_URL)")
		return 2
	}
	// Fail before connecting: without a KEK every row would be left unreadable.
	if !crypto.FieldEncryptionConfigured() {
		fmt.Fprintln(os.Stderr, "reencrypt-worker: ENCRYPTION_KEY_CURRENT (or legacy ENCRYPTION_KEY) is required")
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reencrypt-worker: connect: %v\n", err)
		return 2
	}
	defer pool.Close()
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	encryptor := crypto.GlobalFieldEncryptor()
	if encryptor == nil {
		fmt.Fprintln(os.Stderr, "reencrypt-worker: field encryptor is not initialized")
		return 1
	}

	log := slog.Default()
	tick := func(ctx context.Context) (crypto.ReencryptStats, crypto.ReencryptStats, error) {
		job := crypto.NewReencryptJob(sqlDB, encryptor, *checkpoint)
		job.SetBatchSize(*batch)
		job.SetMaxRowsPerRun(*maxRows)
		columns, err := job.RunFor(ctx, crypto.DefaultEncryptedColumns)
		if err != nil {
			return columns, crypto.ReencryptStats{}, err
		}
		sealed := crypto.ReencryptStats{}
		if *includeSealed {
			sealed, err = job.RunForSealedPayloads(ctx, crypto.SealedPayloadColumns)
			if err != nil {
				return columns, sealed, err
			}
		}
		return columns, sealed, nil
	}

	runOnce := func(ctx context.Context) error {
		start := time.Now()
		columns, sealed, err := tick(ctx)
		log.Info("reencrypt worker tick",
			"duration", time.Since(start).String(),
			"scanned", columns.Scanned, "updated", columns.Updated, "skipped", columns.Skipped,
			"errors", columns.Errors, "completed", columns.Completed,
			"sealed_scanned", sealed.Scanned, "sealed_updated", sealed.Updated,
			"mode", modeLabel(*once))
		if err != nil {
			return err
		}
		if columns.Errors > 0 || sealed.Errors > 0 {
			return fmt.Errorf("reencrypt worker: %d column rows and %d sealed payloads failed", columns.Errors, sealed.Errors)
		}
		return nil
	}

	if *once {
		if err := runOnce(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "reencrypt-worker: %v\n", err)
			return 1
		}
		return 0
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		if err := runOnce(ctx); err != nil {
			log.Error("reencrypt worker tick failed", "error", err)
		}
		select {
		case <-ctx.Done():
			log.Info("reencrypt worker stopped")
			return 0
		case <-ticker.C:
		}
	}
}

func modeLabel(once bool) string {
	if once {
		return "once"
	}
	return "loop"
}

func defaultCheckpointPath() string {
	if v := os.Getenv("REENCRYPT_WORKER_CHECKPOINT"); v != "" {
		return v
	}
	return "reencrypt-worker.checkpoint"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
