package sessions

import (
	"time"

	appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type SessionListItemResponse struct {
	ID               string     `json:"id"`
	UserID           string     `json:"userId"`
	Username         string     `json:"username"`
	OrganizationID   string     `json:"organizationId"`
	OrganizationSlug string     `json:"organizationSlug"`
	ApplicationID    string     `json:"applicationId"`
	ApplicationSlug  string     `json:"applicationSlug"`
	SessionState     string     `json:"sessionState"`
	IPAddress        string     `json:"ipAddress"`
	UserAgent        string     `json:"userAgent"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	RevokedAt        *time.Time `json:"revokedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	LastSeenAt       time.Time  `json:"lastSeenAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type ListSessionsResponse struct {
	Items []SessionListItemResponse `json:"items"`
	Page  presenter.PageMeta        `json:"page"`
}

type SessionResponse struct {
	SessionListItemResponse
	DeviceInfo map[string]any `json:"deviceInfo"`
	Revision   int64          `json:"revision"`
}

type RevokeOtherSessionsResponse struct {
	Status       string `json:"status"`
	RevokedCount int64  `json:"revokedCount"`
}

// ForceLogoutResponse is returned by the administrative force-logout endpoints.
// AC-RISK-002: callers audit session + refresh + access counts independently.
type ForceLogoutResponse struct {
	Status       string `json:"status"`
	RevokedCount int64  `json:"revokedCount"`
	RefreshCount int64  `json:"refreshCount"`
	AccessCount  int64  `json:"accessCount"`
}

func newListSessionsResponse(items []appsessions.SessionListItem, page presenter.PageMeta) *ListSessionsResponse {
	responseItems := make([]SessionListItemResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, toSessionListItemResponse(item))
	}

	return &ListSessionsResponse{Items: responseItems, Page: page}
}

func newSessionResponse(detail *appsessions.SessionDetail) *SessionResponse {
	if detail == nil {
		return nil
	}

	return &SessionResponse{
		SessionListItemResponse: toSessionListItemResponse(detail.SessionListItem),
		DeviceInfo:              detail.DeviceInfo,
		Revision:                detail.Revision,
	}
}

func toSessionListItemResponse(item appsessions.SessionListItem) SessionListItemResponse {
	return SessionListItemResponse{
		ID:               item.ID,
		UserID:           item.UserID,
		Username:         item.Username,
		OrganizationID:   item.OrganizationID,
		OrganizationSlug: item.OrganizationSlug,
		ApplicationID:    item.ApplicationID,
		ApplicationSlug:  item.ApplicationSlug,
		SessionState:     item.SessionState,
		IPAddress:        item.IPAddress,
		UserAgent:        item.UserAgent,
		ExpiresAt:        item.ExpiresAt,
		RevokedAt:        item.RevokedAt,
		CreatedAt:        item.CreatedAt,
		LastSeenAt:       item.LastSeenAt,
		UpdatedAt:        item.UpdatedAt,
	}
}
