// Package usernotifications contains the per-user in-app notification inbox.
//
// 与 domain/notifications（外发投递台账）区分：
//   - notifications 记录一次对外渠道投递（provider/status/retry）；
//   - usernotifications 记录用户在通知中心看到的消息（已读/分类/回复）。
package usernotifications

import (
	"context"
	"time"
)

// Category is the inbox grouping shown in the notification center.
type Category string

const (
	CategorySystem       Category = "system"
	CategorySecurity     Category = "security"
	CategoryBusiness     Category = "business"
	CategoryFeedback     Category = "feedback"
	CategoryAnnouncement Category = "announcement"
)

// Channel is an outbound delivery channel.
type Channel string

const (
	ChannelInApp          Channel = "inapp"
	ChannelEmail          Channel = "email"
	ChannelSMS            Channel = "sms"
	ChannelWeComApp       Channel = "wecom_app"
	ChannelWeChatOfficial Channel = "wechat_official"
)

// Notification is one row in a user's inbox.
type Notification struct {
	ID             string
	OrganizationID string
	UserID         string
	Category       string
	EventType      string
	Title          string
	Content        string
	Link           string
	SourceType     string
	SourceID       *string
	AllowReply     bool
	IsRead         bool
	ReadAt         *time.Time
	DeletedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// Reply is a user reply attached to an inbox notification.
type Reply struct {
	ID             string
	NotificationID string
	OrganizationID string
	UserID         string
	Content        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// Preference toggles one (category, channel) pair for a user.
type Preference struct {
	ID             string
	OrganizationID string
	UserID         string
	Category       string
	Channel        string
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// ChannelBinding maps a user to an external channel identity (WeCom userid,
// WeChat official-account openid, or an alternate mobile number).
type ChannelBinding struct {
	ID             string
	OrganizationID string
	UserID         string
	Channel        string
	ExternalID     string
	Verified       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// ListFilter narrows the inbox query. IsRead nil means both states.
type ListFilter struct {
	Category string
	IsRead   *bool
	Keyword  string
	Page     int
	PageSize int
}

// Repository is the persistence port for the inbox. Every request-scoped
// method takes organizationID and userID; implementations must filter by both
// explicitly (MySQL has no RLS).
type Repository interface {
	List(ctx context.Context, organizationID, userID string, filter ListFilter) ([]*Notification, int64, error)
	GetForUser(ctx context.Context, id, organizationID, userID string) (*Notification, error)
	UnreadCount(ctx context.Context, organizationID, userID string) (int64, error)
	// ExistsBySource 用于事件驱动写入的幂等判定：同一条外发事件重投时，
	// 已经生成过的收件箱消息不应重复创建。
	ExistsBySource(ctx context.Context, organizationID, userID, sourceType, sourceID string) (bool, error)
	Create(ctx context.Context, notification *Notification) error
	MarkRead(ctx context.Context, id, organizationID, userID string, read bool, at time.Time) error
	MarkAllRead(ctx context.Context, organizationID, userID, category string, at time.Time) (int64, error)
	SoftDelete(ctx context.Context, id, organizationID, userID string, at time.Time) error
	BatchSoftDelete(ctx context.Context, ids []string, organizationID, userID string, at time.Time) (int64, error)
	ListReplies(ctx context.Context, notificationID, organizationID, userID string) ([]*Reply, error)
	CreateReply(ctx context.Context, reply *Reply) error
	ListPreferences(ctx context.Context, organizationID, userID string) ([]*Preference, error)
	UpsertPreferences(ctx context.Context, organizationID, userID string, prefs []*Preference) error
	ListChannelBindings(ctx context.Context, organizationID, userID string) ([]*ChannelBinding, error)
	UpsertChannelBinding(ctx context.Context, binding *ChannelBinding) error
	DeleteChannelBinding(ctx context.Context, id, organizationID, userID string) error
}
