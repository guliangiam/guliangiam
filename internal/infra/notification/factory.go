package notification

import (
	"context"
	"sync"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domainnotifications "github.com/guliangiam/guliangiam/internal/domain/notifications"
)

// ProviderBuilder constructs a notifier from one delivery's metadata. Provider
// records whose endpoint or credentials come from the row (custom HTTP, Google
// Chat, CUCloud) cannot be registered as a shared instance at startup.
type ProviderBuilder func(n *Notification) (Provider, error)

// Factory creates and manages notification providers.
type Factory struct {
	mu        sync.RWMutex
	providers map[string]Provider
	builders  map[string]ProviderBuilder
}

// NewFactory creates a new notification provider factory.
func NewFactory() *Factory {
	return &Factory{
		providers: make(map[string]Provider),
		builders:  make(map[string]ProviderBuilder),
	}
}

// Register registers a notification provider under the given type name.
func (f *Factory) Register(providerType string, p Provider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers[providerType] = p
}

// RegisterBuilder registers a per-record constructor. It wins over a shared
// instance registered under the same type.
func (f *Factory) RegisterBuilder(providerType string, builder ProviderBuilder) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.builders[providerType] = builder
}

// GetProvider returns the notification provider for the given type.
func (f *Factory) GetProvider(providerType string) (Provider, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	p, ok := f.providers[providerType]
	if !ok {
		return nil, apperrors.NewUnavailablef("notification factory: provider %q not registered", providerType)
	}
	return p, nil
}

func (f *Factory) sendWith(providerType string, n *Notification) (Provider, error) {
	f.mu.RLock()
	builder := f.builders[providerType]
	f.mu.RUnlock()
	if builder != nil {
		return builder(n)
	}
	return f.GetProvider(providerType)
}

func (f *Factory) Send(ctx context.Context, providerType string, n *Notification) error {
	p, err := f.sendWith(providerType, n)
	if err != nil {
		return err
	}
	return p.Send(ctx, n)
}

var _ domainnotifications.NotificationFactory = (*Factory)(nil)

func (f *Factory) SendNotification(ctx context.Context, providerType string, n *domainnotifications.SendNotification) error {
	infraN := &Notification{
		Type:           n.Type,
		OrganizationID: n.OrganizationID,
		Recipient:      n.Recipient,
		Subject:        n.Subject,
		Content:        n.Content,
		Metadata:       n.Metadata,
	}
	return f.Send(ctx, providerType, infraN)
}
