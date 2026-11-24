package messaging

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domainverifications "github.com/guliangiam/guliangiam/internal/domain/verifications"
)

type SenderFactory struct {
	emailSenders map[string]EmailSender
	smsSenders   map[string]SmsSender
}

func NewSenderFactory() *SenderFactory {
	return &SenderFactory{
		emailSenders: make(map[string]EmailSender),
		smsSenders:   make(map[string]SmsSender),
	}
}

func (f *SenderFactory) RegisterEmailSender(providerType string, sender EmailSender) {
	f.emailSenders[strings.ToLower(providerType)] = sender
}

func (f *SenderFactory) RegisterSmsSender(providerType string, sender SmsSender) {
	f.smsSenders[strings.ToLower(providerType)] = sender
}

func (f *SenderFactory) GetEmailSender(providerType string) (EmailSender, error) {
	sender, ok := f.emailSenders[strings.ToLower(providerType)]
	if !ok {
		return nil, apperrors.NewNotFoundf("email sender not found for provider type: %s", providerType)
	}
	return sender, nil
}

func (f *SenderFactory) GetSmsSender(providerType string) (SmsSender, error) {
	sender, ok := f.smsSenders[strings.ToLower(providerType)]
	if !ok {
		return nil, apperrors.NewNotFoundf("sms sender not found for provider type: %s", providerType)
	}
	return sender, nil
}

var _ domainverifications.SenderFactory = (*SenderFactory)(nil)
