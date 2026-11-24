package notification

import (
	"context"
	"testing"

	domainnotifications "github.com/guliangiam/guliangiam/internal/domain/notifications"
	"github.com/stretchr/testify/assert"
)

type mockProvider struct {
	sendErr error
}

func (m *mockProvider) Send(ctx context.Context, n *Notification) error { return m.sendErr }

func TestNewNotificationFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
}

func TestNotificationFactory_RegisterAndGet(t *testing.T) {
	f := NewFactory()
	p := &mockProvider{}
	f.Register("email", p)

	got, err := f.GetProvider("email")
	assert.NoError(t, err)
	assert.Equal(t, p, got)
}

func TestNotificationFactory_GetProvider_NotFound(t *testing.T) {
	f := NewFactory()
	_, err := f.GetProvider("email")
	assert.Error(t, err)
}

func TestNotificationFactory_Send(t *testing.T) {
	f := NewFactory()
	p := &mockProvider{}
	f.Register("email", p)

	err := f.Send(context.Background(), "email", &Notification{
		Type:      "email",
		Recipient: "test@example.com",
		Subject:   "Test",
		Content:   "Hello",
	})
	assert.NoError(t, err)
}

func TestNotificationFactory_Send_ProviderNotFound(t *testing.T) {
	f := NewFactory()
	err := f.Send(context.Background(), "email", &Notification{})
	assert.Error(t, err)
}

func TestNotificationFactory_SendNotification(t *testing.T) {
	f := NewFactory()
	p := &mockProvider{}
	f.Register("email", p)

	err := f.SendNotification(context.Background(), "email", &domainnotifications.SendNotification{
		Type:      "email",
		Recipient: "test@example.com",
		Subject:   "Test",
		Content:   "Hello",
	})
	assert.NoError(t, err)
}

func TestNotificationFactory_BuilderWinsOverRegisteredInstance(t *testing.T) {
	f := NewFactory()
	instance := &mockProvider{sendErr: assert.AnError}
	f.Register("customhttp", instance)

	var built *Notification
	f.RegisterBuilder("customhttp", func(n *Notification) (Provider, error) {
		built = n
		return &mockProvider{}, nil
	})

	err := f.Send(context.Background(), "customhttp", &Notification{
		Recipient: "https://example.com/hook",
		Content:   "hello",
		Metadata:  map[string]any{"webhook_url": "https://example.com/hook"},
	})
	assert.NoError(t, err, "the per-record builder must win over the startup instance")
	assert.NotNil(t, built)
	assert.Equal(t, "https://example.com/hook", built.Metadata["webhook_url"])
}

func TestNotificationFactory_BuilderErrorPropagates(t *testing.T) {
	f := NewFactory()
	f.RegisterBuilder("googlechat", func(*Notification) (Provider, error) {
		return nil, assert.AnError
	})

	err := f.Send(context.Background(), "googlechat", &Notification{})
	assert.ErrorIs(t, err, assert.AnError)
}

func TestNotificationFactory_GetProviderDoesNotUseBuilder(t *testing.T) {
	f := NewFactory()
	f.RegisterBuilder("cucloud", func(*Notification) (Provider, error) {
		return &mockProvider{}, nil
	})

	_, err := f.GetProvider("cucloud")
	assert.Error(t, err, "GetProvider is the instance-only lookup")
}
