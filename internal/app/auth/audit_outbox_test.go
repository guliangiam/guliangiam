package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
)

type auditPlainTestUOW struct {
	appuow.UnitOfWork
	records domainrecords.Repository
	outbox  domainoutbox.Repository
}

func (u *auditPlainTestUOW) Records() domainrecords.Repository { return u.records }
func (u *auditPlainTestUOW) Outbox() domainoutbox.Repository   { return u.outbox }

type auditPlainRecords struct {
	domainrecords.Repository
	created []*domainrecords.Record
}

func (r *auditPlainRecords) Create(_ context.Context, record *domainrecords.Record) error {
	r.created = append(r.created, record)
	return nil
}

type auditPlainOutbox struct {
	domainoutbox.Repository
	events []*domainoutbox.OutboxEvent
}

func (r *auditPlainOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.events = append(r.events, event)
	return nil
}

func TestEmbedAuditOutboxPlainDropsRequestAndTraceIDs(t *testing.T) {
	records := &auditPlainRecords{}
	outbox := &auditPlainOutbox{}
	uow := &auditPlainTestUOW{records: records, outbox: outbox}

	svc := Service{Clock: clock.SystemClock{}}
	err := svc.embedAuditOutboxPlain(
		uow, context.Background(),
		"1.2.3.4", "test-agent", "protocol",
		"org-1", "user-1", "auth.consent.granted", "consent", "consent-1",
		"consent", "auth.consent.granted",
		map[string]any{"granted": true},
	)
	require.NoError(t, err)
	require.Len(t, records.created, 1)
	require.Len(t, outbox.events, 1)

	record := records.created[0]
	assert.Equal(t, "1.2.3.4", record.IP)
	assert.Equal(t, "test-agent", record.UserAgent)
	assert.Equal(t, "protocol", record.Surface)
	assert.Equal(t, "", record.RequestID, "别名不透传 requestID")
	assert.Equal(t, "", record.TraceID, "别名不透传 traceID")
	assert.Equal(t, "org-1", record.OrganizationID)
	assert.Equal(t, "user-1", record.ActorID)
	assert.Equal(t, "auth.consent.granted", record.Action)
	assert.Equal(t, "consent", record.ResourceType)
	assert.Equal(t, "consent-1", record.ResourceID)

	event := outbox.events[0]
	assert.Equal(t, "consent", event.AggregateType)
	assert.Equal(t, "consent-1", event.AggregateID)
	assert.Equal(t, "auth.consent.granted", event.EventType)
}
