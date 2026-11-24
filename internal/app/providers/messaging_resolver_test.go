package providers

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	domainverifications "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMessagingLookup struct {
	config  *domain.MessagingProviderConfig
	err     error
	org     string
	channel domain.MessagingChannel
}

func (f *fakeMessagingLookup) FindActiveMessagingProviderByChannel(_ context.Context, organizationID string, channel domain.MessagingChannel) (*domain.MessagingProviderConfig, error) {
	f.org, f.channel = organizationID, channel
	return f.config, f.err
}

type fakeSenderBuilder struct {
	sms   domainverifications.SmsSender
	email domainverifications.EmailSender
	err   error
	seen  *domain.MessagingProviderConfig
}

func (f *fakeSenderBuilder) BuildSmsSender(_ context.Context, config *domain.MessagingProviderConfig) (domainverifications.SmsSender, error) {
	f.seen = config
	return f.sms, f.err
}

func (f *fakeSenderBuilder) BuildEmailSender(_ context.Context, config *domain.MessagingProviderConfig) (domainverifications.EmailSender, error) {
	f.seen = config
	return f.email, f.err
}

type fakeSenderFactory struct {
	email         domainverifications.EmailSender
	sms           domainverifications.SmsSender
	emailErr      error
	smsErr        error
	emailProvider string
	smsProvider   string
}

func (f *fakeSenderFactory) GetEmailSender(providerType string) (domainverifications.EmailSender, error) {
	f.emailProvider = providerType
	return f.email, f.emailErr
}

func (f *fakeSenderFactory) GetSmsSender(providerType string) (domainverifications.SmsSender, error) {
	f.smsProvider = providerType
	return f.sms, f.smsErr
}

type fakeSmsSender struct{}

func (fakeSmsSender) Send(context.Context, string, string) error { return nil }

type fakeEmailSender struct{}

func (fakeEmailSender) Send(context.Context, string, string, string) error { return nil }

func TestMessagingResolver_ResolveSmsUsesOrganizationChannelProvider(t *testing.T) {
	lookup := &fakeMessagingLookup{config: &domain.MessagingProviderConfig{
		ProviderID: "provider-1", OrganizationID: "org-1", Kind: "twilio_sms", Channel: domain.MessagingChannelSMS,
		Values: map[string]string{"admin": "sid", "password": "token", "sender": "+10086"},
	}}
	builder := &fakeSenderBuilder{sms: fakeSmsSender{}}
	resolver := MessagingResolver{Lookup: lookup, Builder: builder}

	sender, err := resolver.ResolveSmsSender(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, sender)
	assert.Equal(t, "org-1", lookup.org)
	assert.Equal(t, domain.MessagingChannelSMS, lookup.channel)
	assert.Equal(t, lookup.config, builder.seen)
	decorated, ok := sender.(domainverifications.ProviderIDSmsSender)
	require.True(t, ok, "organization sender must expose the provider row id for delivery traceability")
	assert.Equal(t, "provider-1", decorated.ProviderID())
}

func TestMessagingResolver_ResolveEmailUsesOrganizationChannelProvider(t *testing.T) {
	lookup := &fakeMessagingLookup{config: &domain.MessagingProviderConfig{
		ProviderID: "provider-2", OrganizationID: "org-1", Kind: "smtp", Channel: domain.MessagingChannelEmail,
	}}
	builder := &fakeSenderBuilder{email: fakeEmailSender{}}
	resolver := MessagingResolver{Lookup: lookup, Builder: builder}

	sender, err := resolver.ResolveEmailSender(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, sender)
	assert.Equal(t, domain.MessagingChannelEmail, lookup.channel)
	decorated, ok := sender.(domainverifications.ProviderIDEmailSender)
	require.True(t, ok, "organization sender must expose the provider row id for delivery traceability")
	assert.Equal(t, "provider-2", decorated.ProviderID())
}

func TestMessagingResolver_FallsBackOnlyWhenOrganizationHasNoProvider(t *testing.T) {
	lookup := &fakeMessagingLookup{err: domain.ErrMessagingProviderUnconfigured}
	builder := &fakeSenderBuilder{}
	fallback := &fakeSenderFactory{sms: fakeSmsSender{}, email: fakeEmailSender{}}
	resolver := MessagingResolver{
		Lookup: lookup, Builder: builder, Fallback: fallback,
		FallbackEmailProvider: "smtp", FallbackSmsProvider: "twilio",
	}

	smsSender, err := resolver.ResolveSmsSender(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, smsSender)
	assert.Equal(t, "twilio", fallback.smsProvider)

	emailSender, err := resolver.ResolveEmailSender(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, emailSender)
	assert.Equal(t, "smtp", fallback.emailProvider)
}

func TestMessagingResolver_DisabledProviderDoesNotFallBack(t *testing.T) {
	lookup := &fakeMessagingLookup{err: apperrors.NewUnavailable("sms provider is disabled")}
	builder := &fakeSenderBuilder{}
	fallback := &fakeSenderFactory{sms: fakeSmsSender{}}
	resolver := MessagingResolver{
		Lookup: lookup, Builder: builder, Fallback: fallback,
		FallbackSmsProvider: "twilio",
	}

	_, err := resolver.ResolveSmsSender(context.Background(), "org-1")
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, 503, appErr.HTTPStatus())
	assert.Empty(t, fallback.smsProvider, "disabled provider must never fall through to the deployment sender")
}

func TestMessagingResolver_NoProviderAndNoFallbackIsUnavailable(t *testing.T) {
	resolver := MessagingResolver{
		Lookup:  &fakeMessagingLookup{err: domain.ErrMessagingProviderUnconfigured},
		Builder: &fakeSenderBuilder{},
	}
	_, err := resolver.ResolveSmsSender(context.Background(), "org-1")
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, 503, appErr.HTTPStatus())
}

func TestMessagingResolver_RequiresOrganization(t *testing.T) {
	resolver := MessagingResolver{Lookup: &fakeMessagingLookup{}, Builder: &fakeSenderBuilder{}}
	_, err := resolver.ResolveSmsSender(context.Background(), "")
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, 400, appErr.HTTPStatus())
}

func TestMessagingResolver_UnconfiguredIsInternal(t *testing.T) {
	_, err := (MessagingResolver{}).ResolveSmsSender(context.Background(), "org-1")
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, 500, appErr.HTTPStatus())
}

func TestMessagingKindsAllResolveToAChannel(t *testing.T) {
	kinds := domain.MessagingKinds()
	require.NotEmpty(t, kinds)
	for _, kind := range kinds {
		channel, ok := domain.MessagingChannelForKind(kind)
		assert.True(t, ok, "kind %s has no channel", kind)
		assert.NotEmpty(t, channel)
	}
}
