package messages

import (
	"context"

	appmessages "github.com/guliangiam/guliangiam/internal/app/messages"
)

// Handler for direct message sending endpoints.
type Handler struct {
	Service appmessages.Service
}

// SendEmailRequest is the request body for POST /api/v1/messages/email.
type SendEmailRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// SendSmsRequest is the request body for POST /api/v1/messages/sms.
type SendSmsRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

// SendNotificationRequest is the request body for POST /api/v1/messages/notification.
type SendNotificationRequest struct {
	OrganizationID string   `json:"organizationId"`
	ProviderID     string   `json:"providerId,omitempty"`
	Title          string   `json:"title"`
	Content        string   `json:"content"`
	Targets        []string `json:"targets"`
}

// SendEmail dispatches an email via the configured notification provider.
func (h Handler) SendEmail(ctx context.Context, req SendEmailRequest) error {
	return h.Service.SendEmail(ctx, req.To, req.Subject, req.Body)
}

// SendSms dispatches an SMS via the configured notification provider.
func (h Handler) SendSms(ctx context.Context, req SendSmsRequest) error {
	return h.Service.SendSms(ctx, req.Phone, req.Code)
}

// SendNotification dispatches a push notification to specified targets.
func (h Handler) SendNotification(ctx context.Context, req SendNotificationRequest) error {
	return h.Service.SendNotification(ctx, req.OrganizationID, req.ProviderID, req.Title, req.Content, req.Targets)
}
