package notifications

import (
	"time"
)

type notificationRow struct {
	ID                 string
	OrganizationID     string
	ProviderID         string
	UserNotificationID string
	Channel            string
	ProviderKind       string
	Recipient          string
	Subject            *string
	Content            string
	TemplateID         *string
	Variables          []byte
	Status             string
	Error              *string
	SentAt             *time.Time
	RetryCount         int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
