package bootstrap

import (
	"context"
	"net/http"
	"strings"

	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	domainpayment "github.com/guliangiam/guliangiam/internal/domain/payments"
	infrapayment "github.com/guliangiam/guliangiam/internal/infra/payment"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// newPaymentConnectionTester returns the dispatcher injected into the unified
// provider service. Stripe is the only payment channel with a real connectivity
// probe here; every other channel fails closed as unsupported instead of being
// reported connected after a config-only check.
func newPaymentConnectionTester(baseURL string, client *http.Client) appproviders.PaymentConnectionTester {
	stripe := infrapayment.NewStripeConnectivityTester(client)
	if strings.TrimSpace(baseURL) != "" {
		stripe.BaseURL = baseURL
	}
	return func(ctx context.Context, provider *domainpayment.PayChannelConfig, plaintext string) error {
		if provider == nil {
			return apperrors.NewInvalidArgument("payment channel config is required")
		}
		if strings.EqualFold(strings.TrimSpace(provider.Channel), "stripe") {
			return stripe.Probe(ctx, provider, plaintext)
		}
		return apperrors.NewUnimplementedf("real connectivity probe is not implemented for payment channel %q", provider.Channel)
	}
}

// wireProviderConnectivityProbes attaches the real probe dispatcher to the
// provider service after the composition root has assembled AppServices.
func wireProviderConnectivityProbes(runtime *Runtime) {
	if runtime == nil || runtime.Services.ProviderService.Unified == nil {
		return
	}
	runtime.Services.ProviderService.Unified.PaymentTester = newPaymentConnectionTester("", nil)
}
