package auditoutbox

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	domainuow "github.com/guliangiam/guliangiam/internal/domain/uow"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type captureRecords struct {
	domainrecords.Repository
	created []*domainrecords.Record
}

func (r *captureRecords) Create(_ context.Context, rec *domainrecords.Record) error {
	r.created = append(r.created, rec)
	return nil
}

type captureOutbox struct {
	domainoutbox.Repository
	enqueued []*domainoutbox.OutboxEvent
}

func (o *captureOutbox) Enqueue(_ context.Context, ev *domainoutbox.OutboxEvent) error {
	o.enqueued = append(o.enqueued, ev)
	return nil
}

type stubTx struct {
	domainuow.UnitOfWork
	records *captureRecords
	outbox  *captureOutbox
}

func (s stubTx) Records() domainrecords.Repository { return s.records }
func (s stubTx) Outbox() domainoutbox.Repository   { return s.outbox }

func embedWith(tx stubTx, clk fixedClock, payload map[string]any) error {
	return EmbedAuditOutboxPlain(clk, tx, context.Background(),
		"127.0.0.1", "test-agent", "api", "req-1", "trace-1",
		"org-1", "actor-1", "user.created", "user", "res-1",
		"user", "user.created", payload)
}

// BE-E-09: an unserializable payload must fail the embed, not enqueue an
// empty audit payload silently.
func TestEmbedAuditOutboxPlain_MarshalErrorPropagates(t *testing.T) {
	rec := &captureRecords{}
	ob := &captureOutbox{}
	clk := fixedClock{now: time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)}

	err := embedWith(stubTx{records: rec, outbox: ob}, clk, map[string]any{"bad": make(chan int)})
	require.Error(t, err)
	assert.Empty(t, ob.enqueued, "no outbox event may be enqueued with an empty payload")
}

// BE-E-09: AvailableAt must come from the injected clock, not time.Now().
func TestEmbedAuditOutboxPlain_UsesInjectedClock(t *testing.T) {
	rec := &captureRecords{}
	ob := &captureOutbox{}
	fixed := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

	err := embedWith(stubTx{records: rec, outbox: ob}, fixedClock{now: fixed}, map[string]any{"k": "v"})
	require.NoError(t, err)
	require.Len(t, ob.enqueued, 1)
	assert.Equal(t, fixed, ob.enqueued[0].AvailableAt)
	require.Len(t, rec.created, 1)
	assert.Equal(t, fixed, rec.created[0].CreatedAt)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(ob.enqueued[0].Payload, &decoded))
	assert.Equal(t, "v", decoded["k"])
}
