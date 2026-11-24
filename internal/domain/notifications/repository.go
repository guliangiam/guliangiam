package notifications

import (
	"context"
)

// Repository defines the interface for notifications persistence operations.
type Repository interface {
	// GetByID is the unscoped, system-scoped lookup reserved for background
	// delivery and repair tooling. Request-scoped by-ID paths must use
	// GetByIDForOrganization so a caller-supplied id cannot cross tenants.
	GetByID(ctx context.Context, id string) (*Notification, error)
	// GetByIDForOrganization returns the notification only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Notification, error)
	List(ctx context.Context, organizationID string, status string, keyword string, page, pageSize int) ([]*Notification, int64, error)
	Create(ctx context.Context, notification *Notification) error
	Update(ctx context.Context, notification *Notification) error
	Delete(ctx context.Context, id string) error
	MarkSent(ctx context.Context, id string) error
	// MarkSentForOrganization marks the notification sent only when it belongs
	// to organizationID; zero affected rows is reported as not found.
	MarkSentForOrganization(ctx context.Context, id, organizationID string) error
	MarkFailed(ctx context.Context, id string, errMsg string) error
	IncrementRetryCount(ctx context.Context, id string) error
	ListPendingRetries(ctx context.Context, limit int) ([]*Notification, error)
}

type SendNotification struct {
	Type           string
	OrganizationID string
	Recipient      string
	Subject        string
	Content        string
	Metadata       map[string]interface{}
}

type NotificationFactory interface {
	SendNotification(ctx context.Context, providerType string, n *SendNotification) error
}
