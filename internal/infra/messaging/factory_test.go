package messaging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockEmailSender struct{}

func (m *mockEmailSender) Send(ctx context.Context, to, subject, body string) error { return nil }

type mockSmsSender struct{}

func (m *mockSmsSender) Send(ctx context.Context, phone, code string) error { return nil }

func TestNewSenderFactory(t *testing.T) {
	f := NewSenderFactory()
	assert.NotNil(t, f)
}

func TestSenderFactory_RegisterAndGetEmailSender(t *testing.T) {
	f := NewSenderFactory()
	s := &mockEmailSender{}
	f.RegisterEmailSender("smtp", s)

	got, err := f.GetEmailSender("smtp")
	assert.NoError(t, err)
	assert.Equal(t, s, got)
}

func TestSenderFactory_GetEmailSender_NotFound(t *testing.T) {
	f := NewSenderFactory()
	_, err := f.GetEmailSender("smtp")
	assert.Error(t, err)
}

func TestSenderFactory_RegisterAndGetSmsSender(t *testing.T) {
	f := NewSenderFactory()
	s := &mockSmsSender{}
	f.RegisterSmsSender("twilio", s)

	got, err := f.GetSmsSender("twilio")
	assert.NoError(t, err)
	assert.Equal(t, s, got)
}

func TestSenderFactory_GetSmsSender_NotFound(t *testing.T) {
	f := NewSenderFactory()
	_, err := f.GetSmsSender("twilio")
	assert.Error(t, err)
}

func TestSenderFactory_CaseInsensitive(t *testing.T) {
	f := NewSenderFactory()
	s := &mockEmailSender{}
	f.RegisterEmailSender("SMTP", s)

	got, err := f.GetEmailSender("smtp")
	assert.NoError(t, err)
	assert.Equal(t, s, got)
}
