package notification

import (
	"context"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/guliangiam/guliangiam/internal/infra/messaging"
)

// EmailNotifier sends notifications via email (reuses messaging infrastructure).
type EmailNotifier struct {
	Sender  messaging.EmailSender
	From    string
	Subject string
}

// NewEmailNotifier creates a new email notifier.
func NewEmailNotifier(sender messaging.EmailSender, from, subject string) *EmailNotifier {
	return &EmailNotifier{Sender: sender, From: from, Subject: subject}
}

func (n *EmailNotifier) Send(ctx context.Context, notif *Notification) error {
	subject := notif.Subject
	if subject == "" {
		subject = n.Subject
	}
	if err := n.Sender.Send(ctx, notif.Recipient, subject, notif.Content); err != nil {
		return apperrors.WrapInternalf(err, "email notification")
	}
	return nil
}
