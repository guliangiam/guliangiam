package providers

import (
	"context"
	"fmt"
	"net/smtp"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/smtpclient"
)

var dialSMTPContext = smtpclient.DialContext

// TestConnectionResult contains the result of a provider connection test.
type TestConnectionResult struct {
	ProviderID string `json:"providerId,omitempty"`
	Connected  bool   `json:"connected"`
	Error      string `json:"error,omitempty"`
	LatencyMs  int64  `json:"latencyMs,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
}

// TestConnection runs the family strategy and records its outcome in the same
// transaction as the lifecycle transition and audit/outbox event. The provider
// row is resolved through a tenant-scoped repository read before the strategy
// runs.
func (s Service) TestConnection(ctx context.Context, id, organizationID, actorID string) (*TestConnectionResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	return s.unified().TestConnection(ctx, id, "", organizationID, actorID)
}

// testEmailConnection tests SMTP connectivity by dialing the mail server and attempting
// to start a TLS session. It does not send an actual email.
func testEmailConnection(ctx context.Context, p *domain.Provider) (*TestConnectionResult, error) {
	if p.Host == "" {
		return &TestConnectionResult{Connected: false, Error: "SMTP host is required"}, nil
	}

	addr := fmt.Sprintf("%s:%d", p.Host, p.Port)
	if p.Port == 0 {
		addr = fmt.Sprintf("%s:25", p.Host)
	}

	client, err := dialSMTPContext(ctx, p.Host, p.Port, nil)
	if err != nil {
		return &TestConnectionResult{
			Connected: false,
			Error:     fmt.Sprintf("failed to connect to SMTP server %s: %v", addr, err),
		}, nil
	}
	defer client.Close()

	// Attempt authentication if credentials are provided
	if p.Admin != "" && p.Password != "" {
		auth := smtp.PlainAuth("", p.Admin, p.Password, p.Host)
		if err := client.Auth(auth); err != nil {
			return &TestConnectionResult{
				Connected: false,
				Error:     fmt.Sprintf("SMTP authentication failed: %v", err),
			}, nil
		}
	}

	return &TestConnectionResult{Connected: true}, nil
}
