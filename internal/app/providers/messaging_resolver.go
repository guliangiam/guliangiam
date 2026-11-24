package providers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	domainverifications "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var (
	_ domainverifications.SenderResolver            = MessagingResolver{}
	_ domainverifications.ApplicationSenderResolver = MessagingResolver{}
)

// MessagingResolver resolves the sender for a delivery channel from the
// organization's configured provider row, and only falls back to the
// deployment-level sender when the organization has no row for that channel.
//
// When ApplicationLookup is set and the caller names an application, the
// application's bound provider is tried first. Only enabled rows participate:
// a binding whose provider row is disabled or deleted reads as unconfigured,
// so resolution falls back to the organization provider and then to the
// deployment sender. A lookup failure other than "unconfigured" surfaces as
// 503 instead of falling back.
//
// The organization row follows the same rule: a disabled row is invisible to
// resolution, which falls back to the deployment sender (or 503 when none).
// Resolved senders carry the provider row id so the verification record can
// name the provider that actually delivered the code.
type MessagingResolver struct {
	Lookup            domain.MessagingProviderLookup
	ApplicationLookup domain.MessagingApplicationLookup
	Builder           domain.MessagingSenderBuilder

	// Fallback is the deployment-level (environment) sender factory.
	Fallback              domainverifications.SenderFactory
	FallbackEmailProvider string
	FallbackSmsProvider   string
}

func (r MessagingResolver) ResolveEmailSender(ctx context.Context, organizationID string) (domainverifications.EmailSender, error) {
	if err := r.validate(organizationID); err != nil {
		return nil, err
	}

	config, source, err := resolveMessagingProviderConfig(ctx, r.Lookup, organizationID, domain.MessagingChannelEmail)
	if err != nil {
		if errors.Is(err, domain.ErrMessagingProviderUnconfigured) {
			sender, fallbackErr := r.fallbackEmail()
			if fallbackErr != nil {
				recordProviderResolution("identity", "email", "missing")
				slog.WarnContext(ctx, "email provider is not configured and no deployment fallback is available",
					"organization_id", organizationID, "channel", "email", "error", fallbackErr)
				return nil, fallbackErr
			}
			recordProviderResolution("identity", "email", "fallback")
			slog.DebugContext(ctx, "email provider resolved from the deployment fallback",
				"organization_id", organizationID, "provider", r.FallbackEmailProvider)
			return sender, nil
		}
		result := providerResolutionFailureResult(err)
		recordProviderResolution("identity", "email", result)
		slog.WarnContext(ctx, "email provider resolution failed",
			"organization_id", organizationID, "channel", "email", "result", result, "error", err)
		return nil, apperrors.WrapUnavailablef(err, "email messaging provider is not available")
	}

	sender, err := r.Builder.BuildEmailSender(ctx, config)
	if err != nil {
		recordProviderResolution("identity", "email", "build_error")
		slog.WarnContext(ctx, "email sender construction failed",
			"organization_id", organizationID, "channel", "email", "kind", config.Kind, "error", err)
		return nil, apperrors.WrapUnavailablef(err, "build email sender for %s", config.Kind)
	}
	recordProviderResolution("identity", "email", string(source))
	return emailSenderWithProvider{inner: sender, providerID: config.ProviderID}, nil
}

func (r MessagingResolver) ResolveSmsSender(ctx context.Context, organizationID string) (domainverifications.SmsSender, error) {
	if err := r.validate(organizationID); err != nil {
		return nil, err
	}

	config, source, err := resolveMessagingProviderConfig(ctx, r.Lookup, organizationID, domain.MessagingChannelSMS)
	if err != nil {
		if errors.Is(err, domain.ErrMessagingProviderUnconfigured) {
			sender, fallbackErr := r.fallbackSms()
			if fallbackErr != nil {
				recordProviderResolution("identity", "sms", "missing")
				slog.WarnContext(ctx, "sms provider is not configured and no deployment fallback is available",
					"organization_id", organizationID, "channel", "sms", "error", fallbackErr)
				return nil, fallbackErr
			}
			recordProviderResolution("identity", "sms", "fallback")
			slog.DebugContext(ctx, "sms provider resolved from the deployment fallback",
				"organization_id", organizationID, "provider", r.FallbackSmsProvider)
			return sender, nil
		}
		result := providerResolutionFailureResult(err)
		recordProviderResolution("identity", "sms", result)
		slog.WarnContext(ctx, "sms provider resolution failed",
			"organization_id", organizationID, "channel", "sms", "result", result, "error", err)
		return nil, apperrors.WrapUnavailablef(err, "sms messaging provider is not available")
	}

	sender, err := r.Builder.BuildSmsSender(ctx, config)
	if err != nil {
		recordProviderResolution("identity", "sms", "build_error")
		slog.WarnContext(ctx, "sms sender construction failed",
			"organization_id", organizationID, "channel", "sms", "kind", config.Kind, "error", err)
		return nil, apperrors.WrapUnavailablef(err, "build sms sender for %s", config.Kind)
	}
	recordProviderResolution("identity", "sms", string(source))
	return smsSenderWithProvider{inner: sender, providerID: config.ProviderID}, nil
}

// ResolveEmailSenderForApplication resolves the email sender for one
// application. An explicit application binding wins; when the application has
// no email binding the organization default (and then the deployment fallback)
// applies, which keeps single-provider tenants working unchanged.
func (r MessagingResolver) ResolveEmailSenderForApplication(ctx context.Context, organizationID, applicationID string) (domainverifications.EmailSender, error) {
	if applicationID == "" || r.ApplicationLookup == nil {
		return r.ResolveEmailSender(ctx, organizationID)
	}
	if err := r.validate(organizationID); err != nil {
		return nil, err
	}

	config, err := r.ApplicationLookup.FindActiveMessagingProviderByApplication(ctx, organizationID, applicationID, domain.MessagingChannelEmail)
	if err != nil {
		if errors.Is(err, domain.ErrMessagingProviderUnconfigured) {
			slog.DebugContext(ctx, "application has no email provider binding; falling back to the organization provider",
				"organization_id", organizationID, "application_id", applicationID)
			return r.ResolveEmailSender(ctx, organizationID)
		}
		result := providerResolutionFailureResult(err)
		recordProviderResolution("identity", "email", "application_"+result)
		slog.WarnContext(ctx, "application email provider resolution failed",
			"organization_id", organizationID, "application_id", applicationID, "result", result, "error", err)
		return nil, apperrors.WrapUnavailablef(err, "email messaging provider bound to the application is not available")
	}

	sender, err := r.Builder.BuildEmailSender(ctx, config)
	if err != nil {
		recordProviderResolution("identity", "email", "application_build_error")
		slog.WarnContext(ctx, "application email sender construction failed",
			"organization_id", organizationID, "application_id", applicationID, "kind", config.Kind, "error", err)
		return nil, apperrors.WrapUnavailablef(err, "build email sender for %s", config.Kind)
	}
	recordProviderResolution("identity", "email", "application")
	return emailSenderWithProvider{inner: sender, providerID: config.ProviderID}, nil
}

// ResolveSmsSenderForApplication is the SMS counterpart of
// ResolveEmailSenderForApplication.
func (r MessagingResolver) ResolveSmsSenderForApplication(ctx context.Context, organizationID, applicationID string) (domainverifications.SmsSender, error) {
	if applicationID == "" || r.ApplicationLookup == nil {
		return r.ResolveSmsSender(ctx, organizationID)
	}
	if err := r.validate(organizationID); err != nil {
		return nil, err
	}

	config, err := r.ApplicationLookup.FindActiveMessagingProviderByApplication(ctx, organizationID, applicationID, domain.MessagingChannelSMS)
	if err != nil {
		if errors.Is(err, domain.ErrMessagingProviderUnconfigured) {
			slog.DebugContext(ctx, "application has no sms provider binding; falling back to the organization provider",
				"organization_id", organizationID, "application_id", applicationID)
			return r.ResolveSmsSender(ctx, organizationID)
		}
		result := providerResolutionFailureResult(err)
		recordProviderResolution("identity", "sms", "application_"+result)
		slog.WarnContext(ctx, "application sms provider resolution failed",
			"organization_id", organizationID, "application_id", applicationID, "result", result, "error", err)
		return nil, apperrors.WrapUnavailablef(err, "sms messaging provider bound to the application is not available")
	}

	sender, err := r.Builder.BuildSmsSender(ctx, config)
	if err != nil {
		recordProviderResolution("identity", "sms", "application_build_error")
		slog.WarnContext(ctx, "application sms sender construction failed",
			"organization_id", organizationID, "application_id", applicationID, "kind", config.Kind, "error", err)
		return nil, apperrors.WrapUnavailablef(err, "build sms sender for %s", config.Kind)
	}
	recordProviderResolution("identity", "sms", "application")
	return smsSenderWithProvider{inner: sender, providerID: config.ProviderID}, nil
}

// providerSender wrappers attach the provider row id to a sender without
// changing the narrow sender interfaces the verification service consumes.
// The inner sender stays reachable so SmsCodeVerifier detection survives.
type emailSenderWithProvider struct {
	inner      domainverifications.EmailSender
	providerID string
}

func (s emailSenderWithProvider) Send(ctx context.Context, to, subject, body string) error {
	return s.inner.Send(ctx, to, subject, body)
}

func (s emailSenderWithProvider) ProviderID() string { return s.providerID }

func (s emailSenderWithProvider) InnerEmailSender() domainverifications.EmailSender { return s.inner }

type smsSenderWithProvider struct {
	inner      domainverifications.SmsSender
	providerID string
}

func (s smsSenderWithProvider) Send(ctx context.Context, to, code string) error {
	return s.inner.Send(ctx, to, code)
}

func (s smsSenderWithProvider) ProviderID() string { return s.providerID }

func (s smsSenderWithProvider) InnerSmsSender() domainverifications.SmsSender { return s.inner }

// resolveMessagingProviderConfig lets an instrumented cache report whether the
// answer came from the cache. Plain lookups are database reads.
func resolveMessagingProviderConfig(ctx context.Context, lookup domain.MessagingProviderLookup, organizationID string, channel domain.MessagingChannel) (*domain.MessagingProviderConfig, messagingResolutionSource, error) {
	if sourced, ok := lookup.(sourcedMessagingLookup); ok {
		return sourced.FindActiveMessagingProviderByChannelWithSource(ctx, organizationID, channel)
	}
	config, err := lookup.FindActiveMessagingProviderByChannel(ctx, organizationID, channel)
	return config, messagingSourceDatabase, err
}

// providerResolutionFailureResult maps a lookup error to the metric vocabulary:
// a disabled row is a stable configuration state (503), anything else is an
// unexpected lookup failure.
func providerResolutionFailureResult(err error) string {
	var appErr *apperrors.Error
	if errors.As(err, &appErr) && appErr.HTTPStatus() == http.StatusServiceUnavailable {
		return "disabled"
	}
	return "error"
}

func (r MessagingResolver) fallbackEmail() (domainverifications.EmailSender, error) {
	if r.Fallback == nil {
		return nil, apperrors.NewUnavailable("no email provider is configured for this organization and no deployment default is available")
	}
	sender, err := r.Fallback.GetEmailSender(r.FallbackEmailProvider)
	if err != nil {
		return nil, apperrors.NewUnavailablef("deployment email provider %q is not configured", r.FallbackEmailProvider)
	}
	return sender, nil
}

func (r MessagingResolver) fallbackSms() (domainverifications.SmsSender, error) {
	if r.Fallback == nil {
		return nil, apperrors.NewUnavailable("no sms provider is configured for this organization and no deployment default is available")
	}
	sender, err := r.Fallback.GetSmsSender(r.FallbackSmsProvider)
	if err != nil {
		return nil, apperrors.NewUnavailablef("deployment sms provider %q is not configured", r.FallbackSmsProvider)
	}
	return sender, nil
}

func (r MessagingResolver) validate(organizationID string) error {
	if r.Lookup == nil || r.Builder == nil {
		return apperrors.NewInternal("messaging resolver is not configured")
	}
	if organizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}
