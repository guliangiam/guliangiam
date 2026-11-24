package messaging

import (
	"context"
	"net/http"
)

func NewMockSmsSender() *MockSmsSender {
	return &MockSmsSender{}
}

// Send logs the SMS message without actually sending it.
func (s *MockSmsSender) Send(ctx context.Context, phone, code string) error {
	// Mock sender — does not actually send SMS.
	return nil
}

// Ensure MockSmsSender implements SmsSender.
var _ SmsSender = (*MockSmsSender)(nil)

// OsonSmsSender sends SMS via OSON.
type OsonSmsSender struct {
	APIURL     string
	APIKey     string
	SenderName string
	client     *http.Client
}

// NewOsonSmsSender creates a new OSON SMS sender.
