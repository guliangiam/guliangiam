package metering

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// TestIngest_QuantityMustBePositive pins BE-C-12: a negative (or zero)
// quantity would flow into the aggregate and shrink used_value — a reported
// "top-up" that bypasses the quota. The entry point must reject it.
func TestIngest_QuantityMustBePositive(t *testing.T) {
	tests := []struct {
		name     string
		quantity int64
		wantErr  bool
	}{
		{name: "negative quantity rejected", quantity: -1000000, wantErr: true},
		{name: "zero quantity rejected", quantity: 0, wantErr: true},
		{name: "positive quantity accepted", quantity: 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &quotaDedupRepo{seen: map[string]bool{}, allowed: true, limit: 1000}
			svc := newQuotaDedupService(repo)
			cmds := ingestCmds("k1")
			cmds[0].Quantity = tt.quantity
			_, _, err := svc.Ingest(context.Background(), "org-1", cmds)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Ingest() error = nil, want non-positive quantity rejection")
				}
				if code := apperrors.FromError(err).Code; code != apperrors.CodeUnprocessableEntity {
					t.Fatalf("error code = %s, want %s", code, apperrors.CodeUnprocessableEntity)
				}
				if len(repo.consumed) != 0 {
					t.Errorf("quota consumed = %v, want none on rejection", repo.consumed)
				}
				return
			}
			if err != nil {
				t.Fatalf("Ingest() error = %v", err)
			}
		})
	}
}

// TestIngest_BatchSizeCapped pins the batch upper bound (BE-C-12).
func TestIngest_BatchSizeCapped(t *testing.T) {
	repo := &quotaDedupRepo{seen: map[string]bool{}, allowed: true, limit: 100000}
	svc := newQuotaDedupService(repo)

	cmds := make([]IngestEventCommand, maxIngestBatchSize+1)
	for i := range cmds {
		cmds[i] = IngestEventCommand{EventType: "ai.usage.tokens", Quantity: 1, IdempotencyKey: fmt.Sprintf("k-%d", i)}
	}
	_, _, err := svc.Ingest(context.Background(), "org-1", cmds)
	if err == nil {
		t.Fatal("Ingest() error = nil, want batch-size rejection")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeInvalidArgument {
		t.Fatalf("error code = %s, want %s", code, apperrors.CodeInvalidArgument)
	}
	if len(repo.consumed) != 0 {
		t.Errorf("quota consumed = %v, want none on rejection", repo.consumed)
	}
}

// fixedClock/fixedUUID pin the injected ports for the ingest path (BE-C-21).
type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

type fixedUUID struct{ id string }

func (g fixedUUID) NewString() (string, error) { return g.id, nil }

// TestIngest_UsesInjectedClockAndUUID pins BE-C-21: Ingest must not bypass the
// injected Clock/UUID ports with time.Now()/uuid.NewString().
func TestIngest_UsesInjectedClockAndUUID(t *testing.T) {
	at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	repo := &quotaDedupRepo{seen: map[string]bool{}, allowed: true, limit: 1000}
	svc := Service{Clock: fixedClock{at: at}, UUID: fixedUUID{id: "event-fixed"}, UOW: &quotaDedupUoW{repo: repo}}

	cmds := ingestCmds("k1")
	cmds[0].OccurredAt = time.Time{}
	if _, _, err := svc.Ingest(context.Background(), "org-1", cmds); err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if repo.lastEvent == nil {
		t.Fatal("no event captured")
	}
	if repo.lastEvent.ID != "event-fixed" {
		t.Errorf("ID = %q, want the injected generator's id", repo.lastEvent.ID)
	}
	if !repo.lastEvent.CreatedAt.Equal(at) || !repo.lastEvent.OccurredAt.Equal(at) {
		t.Errorf("CreatedAt/OccurredAt = %v/%v, want the injected clock %v", repo.lastEvent.CreatedAt, repo.lastEvent.OccurredAt, at)
	}
}

var _ clock.Clock = fixedClock{}
var _ platformuuid.Generator = fixedUUID{}
