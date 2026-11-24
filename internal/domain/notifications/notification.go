package notifications

import (
	"context"
	"time"
)

// Notification represents a notification record.
type Notification struct {
	ID             string
	OrganizationID string
	ProviderID     string
	// UserNotificationID links a channel delivery back to the personal inbox
	// row that triggered it. Empty for admin-authored one-off sends.
	UserNotificationID string
	// Channel is the logical delivery channel (email/sms/wecom_app/wechat_official).
	Channel string
	// ProviderKind records which concrete provider adapter handled the row.
	ProviderKind string
	Recipient    string
	Subject      string
	Content      string
	TemplateID   string
	Variables    map[string]any
	Status       string // pending, sent, failed
	Error        string
	SentAt       *time.Time
	RetryCount   int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ProviderType defines the type of notification provider.
type ProviderType string

const (
	ProviderEmail    ProviderType = "email"
	ProviderSMS      ProviderType = "sms"
	ProviderDingTalk ProviderType = "dingtalk"
	ProviderWeCom    ProviderType = "wecom"
	ProviderSlack    ProviderType = "slack"
	ProviderWebhook  ProviderType = "webhook"
	ProviderTelegram ProviderType = "telegram"
)

// SendableProvider is the interface that notification providers must implement.
type SendableProvider interface {
	// Send sends a notification to the given recipient.
	Send(ctx context.Context, notification *Notification) error
	// Validate validates the provider configuration.
	Validate() error
	// Type returns the provider type.
	Type() ProviderType
}
