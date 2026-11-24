package applications

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

type auditWrapperTestUOW struct {
	appuow.UnitOfWork
	records domainrecords.Repository
	outbox  domainoutbox.Repository
}

func (u *auditWrapperTestUOW) Records() domainrecords.Repository { return u.records }
func (u *auditWrapperTestUOW) Outbox() domainoutbox.Repository   { return u.outbox }

type auditWrapperRecords struct {
	domainrecords.Repository
	created []*domainrecords.Record
}

func (r *auditWrapperRecords) Create(_ context.Context, record *domainrecords.Record) error {
	r.created = append(r.created, record)
	return nil
}

type auditWrapperOutbox struct {
	domainoutbox.Repository
	events []*domainoutbox.OutboxEvent
}

func (r *auditWrapperOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.events = append(r.events, event)
	return nil
}

func TestEmbedAuditEventInlinesPackageConstants(t *testing.T) {
	records := &auditWrapperRecords{}
	outbox := &auditWrapperOutbox{}
	uow := &auditWrapperTestUOW{records: records, outbox: outbox}

	svc := Service{Clock: clock.SystemClock{}}
	err := svc.embedAuditEvent(
		uow, context.Background(),
		"org-1", "actor-1", "application.created", "app-1", "application.created",
		map[string]any{"name": "app"},
	)
	require.NoError(t, err)
	require.Len(t, records.created, 1)
	require.Len(t, outbox.events, 1)

	record := records.created[0]
	assert.Equal(t, "org-1", record.OrganizationID)
	assert.Equal(t, "actor-1", record.ActorID, "actor 应透传到审计记录")
	assert.Equal(t, "application.created", record.Action)
	assert.Equal(t, "Application", record.ResourceType)
	assert.Equal(t, "app-1", record.ResourceID)
	assert.Equal(t, "", record.IP)
	assert.Equal(t, "", record.UserAgent)
	assert.Equal(t, "", record.Surface)
	assert.Equal(t, "", record.RequestID)
	assert.Equal(t, "", record.TraceID)

	event := outbox.events[0]
	assert.Equal(t, "Application", event.AggregateType)
	assert.Equal(t, "app-1", event.AggregateID)
	assert.Equal(t, "application.created", event.EventType)
	assert.Contains(t, string(event.Payload), `"name":"app"`)
}
