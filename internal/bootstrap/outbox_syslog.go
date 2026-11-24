package bootstrap

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/providers"
	infraaudit "github.com/guliangiam/guliangiam/internal/infra/audit"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

// auditSyslogObserver forwards successfully delivered outbox events to the
// organization's enabled Audit/Syslog provider. It is a best-effort side sink:
// a missing provider is a no-op, a bad provider config is returned to the
// worker to log, and neither condition rolls back the business delivery.
type auditSyslogObserver struct {
	lookup syslogProviderLookup
	log    *slog.Logger

	mu    sync.Mutex
	sinks map[string]auditSyslogSinkEntry
}

type auditSyslogSinkEntry struct {
	revision int64
	sink     *infraaudit.SyslogSink
}

type syslogProviderLookup interface {
	FindAuditSyslogProvider(ctx context.Context, organizationID string) (*providers.Provider, error)
}

type uowSyslogProviderLookup struct {
	uow appuow.UnitOfWork
}

func (l uowSyslogProviderLookup) FindAuditSyslogProvider(ctx context.Context, organizationID string) (*providers.Provider, error) {
	rows, _, err := l.uow.Providers().List(ctx, providers.ListQuery{
		OrganizationID: organizationID,
		Category:       string(providers.CategoryAudit),
		Page:           1,
		PageSize:       10,
	})
	if err != nil {
		return nil, err
	}
	for _, provider := range rows {
		if provider == nil || !provider.Enabled {
			continue
		}
		if strings.EqualFold(string(provider.ProviderType), string(providers.ProviderSyslog)) {
			return provider, nil
		}
	}
	return nil, nil
}

var _ platformoutbox.DeliveryObserver = (*auditSyslogObserver)(nil)

func newAuditSyslogObserver(lookup syslogProviderLookup) *auditSyslogObserver {
	return &auditSyslogObserver{
		lookup: lookup,
		log:    slog.Default().With("component", "audit-syslog-observer"),
		sinks:  make(map[string]auditSyslogSinkEntry),
	}
}

// Observe resolves the organization's Audit/Syslog provider and writes the
// event. Events from organizations without a provider are skipped.
func (o *auditSyslogObserver) Observe(ctx context.Context, item *platformoutbox.OutboxItem) error {
	if o == nil || o.lookup == nil || item == nil || item.OrganizationID == nil || *item.OrganizationID == "" {
		return nil
	}

	provider, err := o.lookup.FindAuditSyslogProvider(ctx, *item.OrganizationID)
	if err != nil {
		return err
	}
	if provider == nil {
		return nil
	}

	sink, err := o.sinkFor(provider)
	if err != nil {
		return err
	}

	payload := map[string]any{}
	if len(item.Payload) > 0 {
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			payload = map[string]any{"raw": string(item.Payload)}
		}
	}

	return sink.Send(ctx, infraaudit.Event{
		Type:       item.EventType,
		ActorID:    "",
		Resource:   item.AggregateType,
		ResourceID: item.AggregateID,
		Payload:    payload,
	})
}

// sinkFor reuses a provider's connection until the row's revision changes. A
// config edit therefore takes effect without keeping stale connections.
func (o *auditSyslogObserver) sinkFor(provider *providers.Provider) (*infraaudit.SyslogSink, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if entry, ok := o.sinks[provider.ID]; ok && entry.revision == provider.Revision {
		return entry.sink, nil
	}
	if entry, ok := o.sinks[provider.ID]; ok && entry.sink != nil {
		_ = entry.sink.Close()
	}

	sink, err := infraaudit.NewSyslogSink(infraaudit.SyslogConfig{
		Host:     provider.Host,
		Port:     provider.Port,
		Protocol: provider.SubType,
		Facility: provider.Template,
		Format:   provider.ContentType,
		Tag:      provider.Name,
	})
	if err != nil {
		delete(o.sinks, provider.ID)
		return nil, err
	}
	o.sinks[provider.ID] = auditSyslogSinkEntry{revision: provider.Revision, sink: sink}
	return sink, nil
}
