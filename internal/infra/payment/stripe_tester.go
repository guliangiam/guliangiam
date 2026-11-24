package payment

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

const (
	// DefaultStripeAPIBaseURL is Stripe's public API origin. It is a field on
	// StripeConnectivityTester so tests can point the probe at httptest.
	DefaultStripeAPIBaseURL = "https://api.stripe.com"
	// StripeProbeTimeout bounds a single balance probe.
	StripeProbeTimeout = 10 * time.Second
)

// StripeConnectivityTester performs the real Stripe connectivity probe behind
// providers.PaymentConnectionTester: GET /v1/balance with the configured secret
// key. A successful call proves the key is accepted; config-only validation
// never reports Connected.
type StripeConnectivityTester struct {
	BaseURL string
	client  *http.Client
}

// NewStripeConnectivityTester builds a tester with the public Stripe origin and
// an SSRF-guarded client. A nil client falls back to platformssrf.
func NewStripeConnectivityTester(client *http.Client) *StripeConnectivityTester {
	if client == nil {
		client = platformssrf.SafeHTTPClient(StripeProbeTimeout)
	}
	return &StripeConnectivityTester{BaseURL: DefaultStripeAPIBaseURL, client: client}
}

// Probe satisfies app/providers.PaymentConnectionTester (same method signature).
// Errors are classified by platform error code: invalid_argument = unusable
// config, unimplemented = channel has no real probe, anything else = probe
// failure. Messages never include the plaintext config or the API key.
func (t *StripeConnectivityTester) Probe(ctx context.Context, provider *domain.PayChannelConfig, plaintext string) error {
	channel := ""
	if provider != nil {
		channel = provider.Channel
	}
	if !strings.EqualFold(strings.TrimSpace(channel), "stripe") {
		return apperrors.NewUnimplementedf("stripe connectivity probe cannot serve payment channel %q", channel)
	}
	apiKey, err := stripeAPIKeyFromConfig(plaintext)
	if err != nil {
		return configFailure(err)
	}
	balanceURL := t.balanceURL()
	if err := platformssrf.ValidateURL(balanceURL); err != nil {
		return configFailure(apperrors.WrapInvalidArgument(err, "stripe api base url is invalid"))
	}

	ctx, cancel := context.WithTimeout(ctx, StripeProbeTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, balanceURL, http.NoBody)
	if err != nil {
		return apperrors.NewInternal("stripe balance probe request could not be built")
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Accept", "application/json")

	response, err := t.client.Do(request)
	if err != nil {
		// The wrapped network error carries method/URL only; the bearer token
		// stays in the request header and never enters the error chain.
		return newTransportErrorf(err, "stripe balance probe request failed")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))

	switch response.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return newUpstreamStatusErrorf(response.StatusCode, response.Header,
			"stripe rejected the api key (status 401)")
	case http.StatusForbidden:
		return newUpstreamStatusErrorf(response.StatusCode, response.Header,
			"stripe denied access to the balance endpoint (status 403)")
	default:
		return newUpstreamStatusErrorf(response.StatusCode, response.Header,
			"stripe balance probe returned status %d", response.StatusCode)
	}
}

func (t *StripeConnectivityTester) balanceURL() string {
	base := strings.TrimRight(strings.TrimSpace(t.BaseURL), "/")
	if base == "" {
		base = DefaultStripeAPIBaseURL
	}
	return base + "/v1/balance"
}

// stripeAPIKeyFromConfig accepts the JSON object shape used by
// PaymentProviderConfig.Config ({apiKey: "sk_..."}), the common secret-key
// aliases, and a bare key string. Absence of a key is a config failure.
func stripeAPIKeyFromConfig(plaintext string) (string, error) {
	trimmed := strings.TrimSpace(plaintext)
	if trimmed == "" {
		return "", apperrors.NewInvalidArgument("stripe payment config is required")
	}
	if !strings.HasPrefix(trimmed, "{") {
		if strings.HasPrefix(trimmed, "sk_") || strings.HasPrefix(trimmed, "rk_") {
			return trimmed, nil
		}
		return "", apperrors.NewInvalidArgument("stripe payment config is missing apiKey")
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return "", apperrors.NewInvalidArgument("stripe payment config must be a JSON object")
	}
	normalized := make(map[string]string, len(raw))
	for name, value := range raw {
		text, ok := value.(string)
		if !ok {
			continue
		}
		normalized[strings.ToLower(strings.ReplaceAll(name, "_", ""))] = strings.TrimSpace(text)
	}
	for _, name := range []string{"apikey", "secretkey"} {
		if key := normalized[name]; key != "" {
			return key, nil
		}
	}
	return "", apperrors.NewInvalidArgument("stripe payment config is missing apiKey")
}
