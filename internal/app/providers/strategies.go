package providers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainai "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	domainnotification "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	domainpayment "github.com/guliangiam/guliangiam/internal/domain/payments"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

var probeAllowlist = []string{"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}

func (s *UnifiedService) defaultStrategies() map[domain.Family]ProviderStrategy {
	return map[domain.Family]ProviderStrategy{
		domain.FamilyIdentity:     &identityStrategy{client: s.HTTPClient, idvProbe: s.IDVProbe},
		domain.FamilyAI:           &aiStrategy{client: s.HTTPClient},
		domain.FamilyPayment:      &paymentStrategy{tester: s.PaymentTester, cipher: s.cipher, now: s.Clock.Now},
		domain.FamilyNotification: &notificationStrategy{client: s.HTTPClient, tester: s.NotificationTester},
	}
}

type identityStrategy struct {
	client   *http.Client
	idvProbe IDVConnectionProbe
}

func (s *identityStrategy) Family() domain.Family { return domain.FamilyIdentity }
func (s *identityStrategy) Validate(provider *Provider) error {
	if err := provider.Registration.Validate(); err != nil {
		return err
	}
	if detail, ok := provider.Detail.(*domain.Provider); ok {
		if err := detail.ValidateEmailRegex(); err != nil {
			return err
		}
	}
	kind := provider.Registration.Kind
	return validateIdentityKind(kind)
}
func (s *identityStrategy) TestConnection(ctx context.Context, provider *Provider) (*TestConnectionResult, error) {
	detail := provider.Detail.(*domain.Provider)
	mode := identityConnectionProbeMode(detail)
	switch mode {
	case connectionProbeSMTP:
		return testEmailConnection(ctx, detail)
	case connectionProbeAlibabaSMS:
		if problem := validateSMSProviderConfig(detail); problem != "" {
			return &TestConnectionResult{
				Connected: false,
				Error:     problem + " (structural validation only, connectivity NOT verified)",
			}, nil
		}
		return probeAlibabaSMS(ctx, s.client, alibabaSMSEndpoint, detail)
	case connectionProbeHTTP:
		return probeHTTP(ctx, s.client, detail.ProviderURL, 5*time.Second)
	case connectionProbeIDV:
		if s.idvProbe == nil {
			return &TestConnectionResult{
				Connected: false,
				Error:     "identity-verification probe is not wired; connectivity NOT verified",
			}, nil
		}
		if err := s.idvProbe(ctx, detail, provider.config); err != nil {
			return &TestConnectionResult{Connected: false, Error: err.Error()}, nil
		}
		return &TestConnectionResult{Connected: true}, nil
	default:
		// No real probe exists for this family/category. Structural checks may
		// still run, but the result is always connected=false and explicitly
		// states that connectivity was not verified.
		if strings.EqualFold(string(detail.Category), "sms") {
			if problem := validateSMSProviderConfig(detail); problem != "" {
				return &TestConnectionResult{
					Connected: false,
					Error:     problem + " (structural validation only, connectivity NOT verified)",
				}, nil
			}
			return &TestConnectionResult{
				Connected: false,
				Error: fmt.Sprintf(
					"structural validation passed; connectivity NOT verified (no real connection probe is implemented for SMS provider type %q)",
					detail.ProviderType,
				),
			}, nil
		}
		if strings.EqualFold(string(detail.Category), "email") {
			if problem := validateEmailProviderConfig(detail); problem != "" {
				return &TestConnectionResult{
					Connected: false,
					Error:     problem + " (structural validation only, connectivity NOT verified)",
				}, nil
			}
			return &TestConnectionResult{
				Connected: false,
				Error: fmt.Sprintf(
					"structural validation passed; connectivity NOT verified (no real connection probe is implemented for email provider type %q)",
					detail.ProviderType,
				),
			}, nil
		}
		return &TestConnectionResult{
			Connected: false,
			Error: fmt.Sprintf(
				"structural validation only; connectivity NOT verified (no real connection probe is implemented for category %q)",
				detail.Category,
			),
		}, nil
	}
}
func (s *identityStrategy) Sanitize(_ *Provider) map[string]any { return nil }
func (s *identityStrategy) BeforeActivate(_ context.Context, _ appuow.UnitOfWork, provider *Provider) error {
	detail := provider.Detail.(*domain.Provider)
	// 启用是全量生效点：PATCH 只校验变更字段，存量里带 userinfo/fragment 的
	// providerUrl/callbackUrl 不会在编辑时被拦下，却会直接进入登录跳转。这里按
	// 写侧同一规则整体复核，拒绝发生在状态/审计/outbox 写入之前。
	if err := validateOptionalURL(detail.ProviderURL, "providerUrl"); err != nil {
		return err
	}
	if err := validateOptionalCallbackURL(detail.CallbackUrl); err != nil {
		return err
	}
	// Third-party OAuth catalogs carry their endpoints in the adapter, so
	// requiring authUrl/tokenUrl would force admins to duplicate them.
	if isThirdPartyOAuthType(string(detail.ProviderType)) {
		if strings.TrimSpace(detail.ClientID) == "" {
			return apperrors.NewInvalidArgument("clientId is required for OAuth/OIDC providers")
		}
		return nil
	}
	// The canonical kind resolved by the control plane is the single source for
	// the category closure in domain.Provider.Validate; no app-layer bypass.
	detail.Kind = provider.Registration.Kind
	return detail.Validate()
}

// validateDelegatedIdentityCreate is the create-side kind gate. It runs after
// newProvider resolved kind/providerType and before any native write, secret
// status row or audit/outbox embed.
func validateDelegatedIdentityCreate(provider *Provider) error {
	return rejectDelegatedIdentityProvider(provider)
}

// validateDelegatedIdentityPatch is the patch-side kind gate. A historical row
// that already belongs to a dedicated subsystem cannot change kind/providerType
// in either direction, and a row outside the set cannot be moved into it. A
// secret rotation on a delegated row is rejected for the same reason. Renaming
// and other non-kind edits stay allowed so the console can still manage what
// exists; disable/delete have their own call sites.
func validateDelegatedIdentityPatch(provider *Provider, command PatchProviderCommand) error {
	if provider == nil || provider.Registration.Family != domain.FamilyIdentity {
		return nil
	}
	currentProviderType := ""
	if detail, ok := provider.Detail.(*domain.Provider); ok {
		currentProviderType = strings.TrimSpace(string(detail.ProviderType))
	}
	currentKind, currentManagedBy, currentDelegated := delegatedIdentityOf(provider)

	nextKind := provider.Registration.Kind
	kindChanged := false
	if command.Kind != nil {
		nextKind = domain.Kind(strings.ToLower(strings.TrimSpace(*command.Kind)))
		kindChanged = nextKind != provider.Registration.Kind
	}
	providerTypeChanged := false
	if command.ProviderType != nil {
		next := strings.TrimSpace(*command.ProviderType)
		providerTypeChanged = !strings.EqualFold(next, currentProviderType)
	}

	// Moving a delegated row out of its dedicated subsystem is as forbidden as
	// moving a unified row in: both desynchronize the registry from the table
	// the runtime actually reads.
	if currentDelegated && (kindChanged || providerTypeChanged) {
		return newDelegatedKindError(currentKind, currentManagedBy)
	}
	if kindChanged {
		if err := rejectDelegatedKind(nextKind); err != nil {
			return err
		}
	}
	if providerTypeChanged {
		next := strings.TrimSpace(*command.ProviderType)
		if err := rejectDelegatedKind(canonicalIdentityKind(next)); err != nil {
			return err
		}
	}
	if patchRotatesSecret(command) && currentDelegated {
		return newDelegatedKindError(currentKind, currentManagedBy)
	}
	return nil
}

// patchRotatesSecret mirrors the secret branches of Patch: clientSecret is
// converted to RotateProviderSecretCommand before the transaction, while
// clientSecret2/password are written directly to the native row.
func patchRotatesSecret(command PatchProviderCommand) bool {
	return command.Secret != nil || command.ClientSecret != nil ||
		command.ClientSecret2 != nil || command.Password != nil
}

type aiStrategy struct{ client *http.Client }

func (s *aiStrategy) Family() domain.Family { return domain.FamilyAI }
func (s *aiStrategy) Validate(provider *Provider) error {
	if err := provider.Registration.Validate(); err != nil {
		return err
	}
	detail := provider.Detail.(*domainai.LlmProvider)
	if detail.APIKeySealed == "" {
		return apperrors.NewInvalidArgument("apiKey is required")
	}
	return detail.Validate()
}
func (s *aiStrategy) TestConnection(ctx context.Context, provider *Provider) (*TestConnectionResult, error) {
	detail := provider.Detail.(*domainai.LlmProvider)
	timeout := time.Duration(max(detail.RequestTimeoutMs, 3000)) * time.Millisecond
	return probeHTTP(ctx, s.client, detail.BaseURL, timeout)
}
func (s *aiStrategy) Sanitize(provider *Provider) map[string]any {
	p := provider.Detail.(*domainai.LlmProvider)
	return map[string]any{
		"providerType": p.ProviderType, "baseUrl": p.BaseURL, metaKeyStatus: p.Status,
		"supportedProtocols": p.SupportedProtocols, "preferredProtocol": p.PreferredProtocol,
		"requestTimeoutMs": p.RequestTimeoutMs, "maxRetries": p.MaxRetries,
		"priority": p.Priority, "weight": p.Weight, "rpmLimit": p.RPMLimit, "tpmLimit": p.TPMLimit,
		"apiKeySet": p.APIKeySealed != "", "apiKeyMasked": maskedCredential(p.APIKeySealed),
		"apiKeyFingerprint": p.APIKeyFingerprint, metaKeyRevision: p.Revision,
	}
}
func (s *aiStrategy) BeforeActivate(_ context.Context, _ appuow.UnitOfWork, provider *Provider) error {
	if err := s.Validate(provider); err != nil {
		return err
	}
	return platformssrf.ValidateURL(provider.Detail.(*domainai.LlmProvider).BaseURL)
}

type paymentStrategy struct {
	tester PaymentConnectionTester
	cipher func() FieldCipher
	now    func() time.Time
}

func (s *paymentStrategy) Family() domain.Family { return domain.FamilyPayment }
func (s *paymentStrategy) Validate(provider *Provider) error {
	if err := provider.Registration.Validate(); err != nil {
		return err
	}
	detail := provider.Detail.(*domainpayment.PayChannelConfig)
	if err := validatePaymentKind(provider.Registration.Kind); err != nil {
		return err
	}
	if err := validatePaymentEnvironment(detail.Environment, provider.Registration.Variant); err != nil {
		return err
	}
	if detail.ConfigSealed == "" {
		return apperrors.NewInvalidArgument("payment config is required")
	}
	return detail.Validate()
}
func (s *paymentStrategy) TestConnection(ctx context.Context, provider *Provider) (*TestConnectionResult, error) {
	detail := provider.Detail.(*domainpayment.PayChannelConfig)
	if s.tester == nil {
		// Config-only validation is not connectivity: without a real probe the
		// result must stay false instead of reporting a fake "connected".
		return &TestConnectionResult{Connected: false, Error: "real payment connectivity probe is not configured"}, nil
	}
	if strings.TrimSpace(detail.ConfigSealed) == "" {
		return &TestConnectionResult{Connected: false, Error: "payment config verification failed: config is not set"}, nil
	}
	plaintext, err := paymentConfigFromSealed(s.cipher(), provider)
	if err != nil {
		slog.DebugContext(ctx, "payment provider config decrypt failed", "provider_id", provider.Registration.ID, "error", err)
		return &TestConnectionResult{Connected: false, Error: "payment config verification failed: sealed config cannot be decrypted"}, nil
	}
	if err := s.tester(ctx, detail, plaintext); err != nil {
		// The API surface only exposes a coarse failure reason, so keep the
		// upstream detail in debug logs for troubleshooting.
		slog.DebugContext(ctx, "payment provider connection test failed", "provider_id", provider.Registration.ID, "error", err)
		if retryable, ok := apperrors.IsRetryable(err); ok && retryable {
			// Rate-limited / temporarily unavailable upstream: surface the
			// classified platform error so route.WriteError can answer
			// 429/503 with error.meta.retryable and the upstream Retry-After.
			// Non-retryable outcomes keep the connected=false envelope.
			return nil, err
		}
		return &TestConnectionResult{Connected: false, Error: paymentProbeFailureReason(err)}, nil
	}
	return &TestConnectionResult{Connected: true}, nil
}

// paymentProbeFailureReason maps the tester's platform error code to the coarse
// reason returned by the API. Config errors and unsupported channels must stay
// distinguishable from a probe that actually ran and failed.
func paymentProbeFailureReason(err error) string {
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case apperrors.CodeInvalidArgument:
			return "payment config verification failed"
		case apperrors.CodeUnimplemented:
			return "real payment connectivity probe is not supported for this channel"
		}
	}
	return "payment connectivity probe failed"
}
func (s *paymentStrategy) Sanitize(provider *Provider) map[string]any {
	p := provider.Detail.(*domainpayment.PayChannelConfig)
	return map[string]any{
		"channel": p.Channel, "environment": p.Environment, metaKeyStatus: p.Status,
		"configSet": p.ConfigSet, "configMasked": maskedCredential(p.ConfigSealed),
		"configFingerprint": p.ConfigFingerprint, "versionNo": p.VersionNo,
		"testPassedAt": p.TestPassedAt, "activatedAt": p.ActivatedAt, "disabledAt": p.DisabledAt,
		metaKeyRevision: p.Revision,
	}
}
func (s *paymentStrategy) BeforeActivate(ctx context.Context, tx appuow.UnitOfWork, provider *Provider) error {
	if err := s.Validate(provider); err != nil {
		return err
	}
	detail := provider.Detail.(*domainpayment.PayChannelConfig)
	if err := validateActivationReadiness(detail.TestPassedAt); err != nil {
		return err
	}
	repo, err := paymentChannelConfigs(tx)
	if err != nil {
		return err
	}
	items, err := repo.List(ctx, detail.OrganizationID)
	if err != nil {
		return err
	}
	now := s.now()
	for _, other := range items {
		if other.ID == detail.ID || other.Channel != detail.Channel || other.Environment != detail.Environment || other.Status != domainpayment.ChannelConfigStatusActive {
			continue
		}
		other.Status = domainpayment.ChannelConfigStatusDisabled
		other.DisabledAt = &now
		other.UpdatedAt = now
		other.Revision++
		if err := repo.Update(ctx, other); err != nil {
			return err
		}
	}
	return nil
}

type notificationStrategy struct {
	client *http.Client
	tester NotificationConnectionTester
}

func (s *notificationStrategy) Family() domain.Family { return domain.FamilyNotification }
func (s *notificationStrategy) Validate(provider *Provider) error {
	if err := provider.Registration.Validate(); err != nil {
		return err
	}
	if err := validateNotificationKind(provider.Registration.Kind); err != nil {
		return err
	}
	detail := provider.Detail.(*domainnotification.NotificationProvider)
	if detail.Name == "" || detail.OrganizationID == "" {
		return apperrors.NewInvalidArgument("notification provider name and organizationId are required")
	}
	if detail.WebhookURL != "" {
		return validateOptionalURL(detail.WebhookURL, "webhookUrl")
	}
	return nil
}
func (s *notificationStrategy) TestConnection(ctx context.Context, provider *Provider) (*TestConnectionResult, error) {
	detail := provider.Detail.(*domainnotification.NotificationProvider)
	if s.tester != nil {
		if err := s.tester(ctx, detail); err != nil {
			// The API surface only exposes a coarse failure reason, so keep the
			// upstream detail in debug logs for troubleshooting.
			slog.DebugContext(ctx, "notification provider connection test failed", "provider_id", provider.Registration.ID, "error", err)
			return &TestConnectionResult{Connected: false, Error: "notification connection test failed"}, nil
		}
		return &TestConnectionResult{Connected: true}, nil
	}
	if detail.WebhookURL == "" {
		return &TestConnectionResult{Connected: false, Error: "notification connection test requires a webhookUrl or configured tester"}, nil
	}
	return probeHTTP(ctx, s.client, detail.WebhookURL, 5*time.Second)
}
func (s *notificationStrategy) Sanitize(provider *Provider) map[string]any {
	p := provider.Detail.(*domainnotification.NotificationProvider)
	return map[string]any{
		"providerType": p.ProviderType, "webhookUrl": p.WebhookURL, "isEnabled": p.IsEnabled,
		"accessTokenSet": p.AccessToken != "", "accessTokenMasked": maskedCredential(p.AccessToken),
		metaKeyConfig: sanitizeConfig(p.Config), metaKeyRevision: p.Revision,
	}
}
func (s *notificationStrategy) BeforeActivate(_ context.Context, _ appuow.UnitOfWork, provider *Provider) error {
	return s.Validate(provider)
}

func maskedCredential(value string) string {
	if value == "" {
		return ""
	}
	return "******"
}

func sanitizeConfig(config map[string]any) map[string]any {
	if config == nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(config))
	for key, value := range config {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "apikey") || strings.Contains(lower, "api_key") || strings.Contains(lower, "privatekey") {
			result[key] = maskedCredential(fmt.Sprint(value))
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			result[key] = sanitizeConfig(nested)
			continue
		}
		result[key] = value
	}
	return result
}

func probeHTTP(ctx context.Context, client *http.Client, rawURL string, timeout time.Duration) (*TestConnectionResult, error) {
	if err := platformssrf.ValidateURL(rawURL); err != nil {
		return nil, apperrors.NewInvalidArgumentf("invalid provider endpoint: %v", err)
	}
	if client == nil {
		client = platformssrf.SafeHTTPClientWithAllowlist(timeout, probeAllowlist)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, http.NoBody)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("invalid provider endpoint")
	}
	start := time.Now()
	response, err := client.Do(request)
	result := &TestConnectionResult{LatencyMs: time.Since(start).Milliseconds()}
	if err != nil {
		result.Error = "provider endpoint is unreachable"
		return result, nil
	}
	defer response.Body.Close()
	result.Connected = true
	result.StatusCode = response.StatusCode
	return result, nil
}
