package audit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	domainuow "github.com/guliangiam/guliangiam/internal/domain/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/requestmeta"
)

type captureRecords struct {
	domainrecords.Repository
	created []*domainrecords.Record
}

func (r *captureRecords) Create(_ context.Context, record *domainrecords.Record) error {
	r.created = append(r.created, record)
	return nil
}

type captureOutbox struct {
	domainoutbox.Repository
	events []*domainoutbox.OutboxEvent
}

func (r *captureOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.events = append(r.events, event)
	return nil
}

type captureUOW struct {
	domainuow.UnitOfWork
	records domainrecords.Repository
	outbox  domainoutbox.Repository
}

func (u *captureUOW) Records() domainrecords.Repository { return u.records }
func (u *captureUOW) Outbox() domainoutbox.Repository   { return u.outbox }

func TestEmbedAuditEventMapsEveryField(t *testing.T) {
	records := &captureRecords{}
	outbox := &captureOutbox{}
	uow := &captureUOW{records: records, outbox: outbox}

	err := EmbedAuditEvent(
		clock.SystemClock{}, uow, context.Background(),
		"1.2.3.4", "test-agent", "management", "req-1", "trace-1",
		"org-1", "actor-1", "user.created", "user", "user-1",
		"user", "user.created",
		map[string]any{"source": "test"},
	)
	require.NoError(t, err)
	require.Len(t, records.created, 1)
	require.Len(t, outbox.events, 1)

	record := records.created[0]
	assert.Equal(t, "org-1", record.OrganizationID)
	assert.Equal(t, "actor-1", record.ActorID)
	assert.Equal(t, "user.created", record.Action)
	assert.Equal(t, "user", record.ResourceType)
	assert.Equal(t, "user-1", record.ResourceID)
	assert.Equal(t, "1.2.3.4", record.IP)
	assert.Equal(t, "test-agent", record.UserAgent)
	assert.Equal(t, "management", record.Surface)
	assert.Equal(t, "req-1", record.RequestID)
	assert.Equal(t, "trace-1", record.TraceID)
	assert.NotEmpty(t, record.ID)
	assert.False(t, record.CreatedAt.IsZero())

	event := outbox.events[0]
	require.NotNil(t, event.OrganizationID)
	assert.Equal(t, "org-1", *event.OrganizationID)
	assert.Equal(t, "user", event.AggregateType)
	assert.Equal(t, "user-1", event.AggregateID)
	assert.Equal(t, "user.created", event.EventType)
	assert.Equal(t, 1, event.EventVersion)
	assert.Contains(t, string(event.Payload), `"source":"test"`)
}

func TestEmbedAuditEventFallsBackToBuiltInOrganization(t *testing.T) {
	records := &captureRecords{}
	outbox := &captureOutbox{}
	uow := &captureUOW{records: records, outbox: outbox}

	err := EmbedAuditEvent(
		clock.SystemClock{}, uow, context.Background(),
		"", "", "management", "", "",
		"", "", "org.created", "organization", "org-1",
		"organization", "org.created",
		map[string]any{},
	)
	require.NoError(t, err)

	const builtInOrgUUID = "01a0ea76-0c00-7000-8000-000000000001"
	assert.Equal(t, builtInOrgUUID, records.created[0].OrganizationID)
	assert.Equal(t, builtInOrgUUID, *outbox.events[0].OrganizationID)
}

func TestEmbedAuditEventSkipsWhenClockIsNil(t *testing.T) {
	records := &captureRecords{}
	outbox := &captureOutbox{}
	uow := &captureUOW{records: records, outbox: outbox}

	err := EmbedAuditEvent(nil, uow, context.Background(),
		"", "", "", "", "", "", "", "x", "x", "x", "x", "x", map[string]any{})
	require.NoError(t, err)
	assert.Empty(t, records.created)
	assert.Empty(t, outbox.events)
}

func TestEmbedAuditEventWritesOutboxWithoutRecordsRepo(t *testing.T) {
	outbox := &captureOutbox{}
	uow := &captureUOW{outbox: outbox}

	err := EmbedAuditEvent(
		clock.SystemClock{}, uow, context.Background(),
		"", "", "", "", "",
		"org-1", "actor-1", "x", "x", "x",
		"x", "x", map[string]any{},
	)
	require.NoError(t, err)
	assert.Len(t, outbox.events, 1)
}

// An application-layer caller passes no request metadata; the values stamped by
// the transport middleware on ctx must fill the blanks.
func TestEmbedAuditEventBackfillsRequestMetadata(t *testing.T) {
	records := &captureRecords{}
	uow := &captureUOW{records: records}
	ctx := requestmeta.With(context.Background(), requestmeta.Metadata{
		IP:        "203.0.113.7",
		UserAgent: "agent/1.0",
		Surface:   "management",
		RequestID: "req-1",
		TraceID:   "trace-1",
	})

	err := EmbedAuditEvent(
		clock.SystemClock{}, uow, ctx,
		"", "", "", "", "",
		"org-1", "actor-1", "application.created", "application", "app-1",
		"application", "application.created",
		map[string]any{},
	)
	require.NoError(t, err)
	require.Len(t, records.created, 1)

	record := records.created[0]
	assert.Equal(t, "203.0.113.7", record.IP)
	assert.Equal(t, "agent/1.0", record.UserAgent)
	assert.Equal(t, "management", record.Surface)
	assert.Equal(t, "req-1", record.RequestID)
	assert.Equal(t, "trace-1", record.TraceID)
}

// Explicit values always win: a protocol event that names its own surface must
// not be relabelled by the ambient request metadata.
func TestEmbedAuditEventKeepsExplicitMetadata(t *testing.T) {
	records := &captureRecords{}
	uow := &captureUOW{records: records}
	ctx := requestmeta.With(context.Background(), requestmeta.Metadata{
		IP:        "203.0.113.7",
		UserAgent: "agent/1.0",
		Surface:   "management",
		RequestID: "req-1",
		TraceID:   "trace-1",
	})

	err := EmbedAuditEvent(
		clock.SystemClock{}, uow, ctx,
		"1.2.3.4", "explicit-agent", "protocol", "req-explicit", "trace-explicit",
		"org-1", "actor-1", "oidc.login", "session", "session-1",
		"session", "oidc.login",
		map[string]any{},
	)
	require.NoError(t, err)
	require.Len(t, records.created, 1)

	record := records.created[0]
	assert.Equal(t, "1.2.3.4", record.IP)
	assert.Equal(t, "explicit-agent", record.UserAgent)
	assert.Equal(t, "protocol", record.Surface)
	assert.Equal(t, "req-explicit", record.RequestID)
	assert.Equal(t, "trace-explicit", record.TraceID)
}
