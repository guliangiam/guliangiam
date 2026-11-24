package usernotifications

import (
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/usernotifications"
)

// NotificationDTO is the API projection of an inbox message.
type NotificationDTO struct {
	ID         string     `json:"id"`
	Category   string     `json:"category"`
	EventType  string     `json:"eventType"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	Link       string     `json:"link"`
	SourceType string     `json:"sourceType"`
	SourceID   *string    `json:"sourceId,omitempty"`
	AllowReply bool       `json:"allowReply"`
	IsRead     bool       `json:"isRead"`
	ReadAt     *time.Time `json:"readAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type NotificationListDTO struct {
	Items    []NotificationDTO `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

type ReplyDTO struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	UserID    string    `json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}

type PreferenceDTO struct {
	Category string `json:"category"`
	Channel  string `json:"channel"`
	Enabled  bool   `json:"enabled"`
}

type ChannelBindingDTO struct {
	ID         string    `json:"id"`
	Channel    string    `json:"channel"`
	ExternalID string    `json:"externalId"`
	Verified   bool      `json:"verified"`
	CreatedAt  time.Time `json:"createdAt"`
}

func toNotificationDTO(item *domain.Notification) NotificationDTO {
	return NotificationDTO{
		ID:         item.ID,
		Category:   item.Category,
		EventType:  item.EventType,
		Title:      item.Title,
		Content:    item.Content,
		Link:       item.Link,
		SourceType: item.SourceType,
		SourceID:   item.SourceID,
		AllowReply: item.AllowReply,
		IsRead:     item.IsRead,
		ReadAt:     item.ReadAt,
		CreatedAt:  item.CreatedAt,
	}
}

func toReplyDTO(item *domain.Reply) ReplyDTO {
	return ReplyDTO{
		ID:        item.ID,
		Content:   item.Content,
		UserID:    item.UserID,
		CreatedAt: item.CreatedAt,
	}
}

func toChannelBindingDTO(item *domain.ChannelBinding) ChannelBindingDTO {
	return ChannelBindingDTO{
		ID:         item.ID,
		Channel:    item.Channel,
		ExternalID: item.ExternalID,
		Verified:   item.Verified,
		CreatedAt:  item.CreatedAt,
	}
}

var validCategories = map[string]bool{
	string(domain.CategorySystem):       true,
	string(domain.CategorySecurity):     true,
	string(domain.CategoryBusiness):     true,
	string(domain.CategoryFeedback):     true,
	string(domain.CategoryAnnouncement): true,
}

var validChannels = map[string]bool{
	string(domain.ChannelInApp):          true,
	string(domain.ChannelEmail):          true,
	string(domain.ChannelSMS):            true,
	string(domain.ChannelWeComApp):       true,
	string(domain.ChannelWeChatOfficial): true,
}
