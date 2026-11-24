package messaging

import (
	"context"
	"log/slog"

	domainverifications "github.com/guliangiam/guliangiam/internal/domain/verifications"
)

// MockEmailSender is a development fallback that logs email messages to stderr
// instead of actually sending them.  It is registered automatically when no
// SMTP/SendGrid/Resend/AzureACS env vars are present so verification code
// flows (email reset, MFA) work out of the box in local development.
type MockEmailSender struct{}

func NewMockEmailSender() *MockEmailSender {
	return &MockEmailSender{}
}

// Send logs the email message without actually sending it.
func (s *MockEmailSender) Send(ctx context.Context, to, subject, body string) error {
	slog.Warn("mock email sender — message logged, not delivered",
		"to", to,
		"subject", subject,
	)
	slog.InfoContext(ctx, "mock email body", "body", body)
	return nil
}

// Ensure MockEmailSender implements domainverifications.EmailSender.
var _ domainverifications.EmailSender = (*MockEmailSender)(nil)
