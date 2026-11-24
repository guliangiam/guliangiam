package captcha

import "context"

// Provider is the interface for CAPTCHA verification providers.
type Provider interface {
	// Verify verifies a CAPTCHA token against the provider's API.
	Verify(ctx context.Context, token string, remoteIP string) (bool, error)
}
