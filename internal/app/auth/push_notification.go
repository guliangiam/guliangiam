package auth

import (
	"context"
	"fmt"
	"time"
)

// MfaPushNotificationParams contains the parameters for building an MFA push notification.
type MfaPushNotificationParams struct {
	ApplicationName string
	LoginTime       time.Time
	IPAddress       string
	ActionType      string
}

// MfaPushNotification represents a structured push notification for MFA.
type MfaPushNotification struct {
	Title string
	Body  string
}

// BuildMfaPushNotification builds an MFA push notification with structured content
// including the application name, login time, IP address, and action type.
func BuildMfaPushNotification(params MfaPushNotificationParams) MfaPushNotification {
	title := "MFA Verification Request"
	if params.ApplicationName != "" {
		title = fmt.Sprintf("MFA Verification - %s", params.ApplicationName)
	}

	body := "An authentication request has been initiated for your account."
	if params.ActionType != "" {
		body = fmt.Sprintf("A %s request has been initiated for your account.", params.ActionType)
	}

	details := ""
	if !params.LoginTime.IsZero() {
		details += fmt.Sprintf("\nTime: %s", params.LoginTime.Format(time.RFC3339))
	}
	if params.IPAddress != "" {
		details += fmt.Sprintf("\nIP: %s", params.IPAddress)
	}
	if details != "" {
		body += details
	}

	body += "\n\nPlease approve or reject this request."

	return MfaPushNotification{
		Title: title,
		Body:  body,
	}
}

// MfaPushNotificationSender extends PushNotifier with MFA-specific notification sending.
type MfaPushNotificationSender interface {
	PushNotifier
	// SendMfaNotification sends an MFA-specific push notification with structured content.
	SendMfaNotification(ctx context.Context, providerType string, config pushProviderConfig, receiver string, params MfaPushNotificationParams) error
}

// mfaPushNotificationSenderImpl implements MfaPushNotificationSender by wrapping a PushNotifier.
type mfaPushNotificationSenderImpl struct {
	delegate PushNotifier
}

// NewMfaPushNotificationSender creates a new MfaPushNotificationSender wrapping the given PushNotifier.
func NewMfaPushNotificationSender(delegate PushNotifier) MfaPushNotificationSender {
	return &mfaPushNotificationSenderImpl{delegate: delegate}
}

func (s *mfaPushNotificationSenderImpl) Send(ctx context.Context, providerType string, config pushProviderConfig, receiver, title, body string) error {
	return s.delegate.Send(ctx, providerType, config, receiver, title, body)
}

func (s *mfaPushNotificationSenderImpl) SendMfaNotification(ctx context.Context, providerType string, config pushProviderConfig, receiver string, params MfaPushNotificationParams) error {
	notification := BuildMfaPushNotification(params)
	return s.delegate.Send(ctx, providerType, config, receiver, notification.Title, notification.Body)
}
