// Package providerlogout delivers upstream OAuth provider logout requests from
// the shared Outbox worker. The request path only enqueues the event, so a slow
// or unavailable provider can never lengthen a user's logout.
package providerlogout

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	providerlogoutevent "github.com/guliangiam/guliangiam/internal/platform/providerlogout"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// AggregateType is the Outbox routing key for this handler.
const AggregateType = providerlogoutevent.AggregateType

const providerLogoutTimeout = 5 * time.Second

// ProviderLoader loads a provider by ID. Bootstrap supplies a loader that runs
// the read in the system transaction scope, because the worker has no request
// tenant context and providers is a FORCE RLS table.
type ProviderLoader interface {
	GetProvider(ctx context.Context, providerID string) (*providerdomain.Provider, error)
}

// TokenDecrypter opens the sealed provider access token kept in the event.
// It matches platform/crypto.FieldEncryptor.
type TokenDecrypter interface {
	DecryptField(ciphertext string) (string, error)
}

// Handler delivers one provider logout per Outbox item.
type Handler struct {
	providers ProviderLoader
	cipher    TokenDecrypter
	client    *http.Client
}

// NewHandler constructs the DeliveryHandler. client == nil builds the
// provider-aware SSRF-safe default client per request.
func NewHandler(providers ProviderLoader, cipher TokenDecrypter, client *http.Client) platformoutbox.DeliveryHandler {
	return &Handler{providers: providers, cipher: cipher, client: client}
}

// Handle loads the provider, opens the token, and performs the RFC 7009-style
// POST. Retry classification:
//
//   - permanent: malformed payload, provider deleted, no decryption key,
//     undecryptable token, 4xx other than 408/429 — retrying cannot help.
//   - retryable: provider read failures, network errors/timeouts, 5xx, 408, 429.
//
// The item ID (aggregate_id = session ID) is the idempotency key; a token
// revocation endpoint is idempotent, so at-least-once redelivery is safe.
func (h *Handler) Handle(ctx context.Context, item *platformoutbox.OutboxItem) error {
	if item == nil {
		return platformoutbox.Permanent(apperrors.NewInvalidArgument("provider logout: outbox item is nil"))
	}
	var event providerlogoutevent.Payload
	if err := json.Unmarshal(item.Payload, &event); err != nil {
		return platformoutbox.Permanent(apperrors.WrapInternalf(err, "provider logout: parse payload"))
	}
	providerID := strings.TrimSpace(event.ProviderID)
	sealedToken := strings.TrimSpace(event.SealedAccessToken)
	if providerID == "" || sealedToken == "" {
		return platformoutbox.Permanent(apperrors.NewInvalidArgument(
			"provider logout: payload must contain providerId and sealedAccessToken"))
	}

	provider, err := h.providers.GetProvider(ctx, providerID)
	if err != nil {
		if apperrors.FromError(err).Code == apperrors.CodeNotFound {
			return platformoutbox.Permanent(apperrors.WrapNotFoundf(err, "provider logout: provider %s no longer exists", providerID))
		}
		return apperrors.WrapInternal(err, "provider logout: load provider")
	}
	if provider == nil || strings.TrimSpace(provider.CustomLogoutUrl) == "" {
		// The provider has no custom logout endpoint configured: nothing to
		// deliver, and nothing will change on retry.
		return nil
	}
	if h.cipher == nil {
		return platformoutbox.Permanent(apperrors.NewInvalidArgument(
			"provider logout: no provider token cipher configured"))
	}
	token, err := h.cipher.DecryptField(sealedToken)
	if err != nil {
		// The sealed value cannot be opened with the current key: a retry would
		// fail identically.
		return platformoutbox.Permanent(apperrors.WrapInternalf(err, "provider logout: open access token"))
	}

	return h.post(ctx, provider, token)
}

// post performs the RFC 7009-style POST against the provider logout endpoint.
func (h *Handler) post(ctx context.Context, provider *providerdomain.Provider, token string) error {
	form := url.Values{}
	form.Set("token", token)
	form.Set("client_id", provider.ClientID)
	form.Set("client_secret", provider.ClientSecret)

	requestCtx, cancel := context.WithTimeout(ctx, providerLogoutTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, provider.CustomLogoutUrl, strings.NewReader(form.Encode()))
	if err != nil {
		return platformoutbox.Permanent(apperrors.WrapInternalf(err, "provider logout: build request"))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for key, value := range provider.HttpHeaders {
		req.Header.Set(key, value)
	}

	client := h.client
	if client == nil {
		if provider.EnableProxy {
			client = ssrf.SafeProxyHTTPClient(providerLogoutTimeout)
		} else {
			client = ssrf.SafeHTTPClient(providerLogoutTimeout)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return apperrors.WrapInternal(err, "provider logout: request failed")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests:
		return apperrors.NewUnavailablef("provider logout: provider returned status %d", resp.StatusCode)
	case resp.StatusCode >= 500:
		return apperrors.NewUnavailablef("provider logout: provider returned status %d", resp.StatusCode)
	default:
		return platformoutbox.Permanent(apperrors.NewInvalidArgumentf(
			"provider logout: provider returned status %d", resp.StatusCode))
	}
}
