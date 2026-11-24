package notification

import (
	"context"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/guliangiam/guliangiam/internal/infra/messaging"
)

// SmsNotifier sends notifications via SMS (reuses messaging infrastructure).
type SmsNotifier struct {
	Sender messaging.SmsSender
}

// NewSmsNotifier creates a new SMS notifier.
func NewSmsNotifier(sender messaging.SmsSender) *SmsNotifier {
	return &SmsNotifier{Sender: sender}
}

func (n *SmsNotifier) Send(ctx context.Context, notif *Notification) error {
	if err := n.Sender.Send(ctx, notif.Recipient, notif.Content); err != nil {
		return apperrors.WrapInternalf(err, "sms notification")
	}
	return nil
}
