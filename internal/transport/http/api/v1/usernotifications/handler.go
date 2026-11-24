package usernotifications

import (
	"context"

	appusernotifications "github.com/guliangiam/guliangiam/internal/app/usernotifications"
)

type scope struct {
	OrganizationID string
	UserID         string
}

func (h Handler) List(ctx context.Context, sc scope, request ListRequest) (*appusernotifications.NotificationListDTO, error) {
	return h.Service.List(ctx, appusernotifications.ListQuery{
		OrganizationID: sc.OrganizationID,
		UserID:         sc.UserID,
		Category:       request.Category,
		IsRead:         request.IsRead,
		Keyword:        request.Keyword,
		Page:           request.Page,
		PageSize:       request.PageSize,
	})
}

func (h Handler) UnreadCount(ctx context.Context, sc scope) (int64, error) {
	return h.Service.UnreadCount(ctx, sc.OrganizationID, sc.UserID)
}

func (h Handler) Get(ctx context.Context, sc scope, id string) (*appusernotifications.NotificationDTO, error) {
	return h.Service.Get(ctx, appusernotifications.IDCommand{
		OrganizationID: sc.OrganizationID,
		UserID:         sc.UserID,
		ID:             id,
	})
}

func (h Handler) MarkRead(ctx context.Context, sc scope, id string, read bool) error {
	return h.Service.MarkRead(ctx, appusernotifications.MarkReadCommand{
		OrganizationID: sc.OrganizationID,
		UserID:         sc.UserID,
		ID:             id,
		Read:           read,
	})
}

func (h Handler) MarkAllRead(ctx context.Context, sc scope, category string) (int64, error) {
	return h.Service.MarkAllRead(ctx, appusernotifications.MarkAllReadCommand{
		OrganizationID: sc.OrganizationID,
		UserID:         sc.UserID,
		Category:       category,
	})
}

func (h Handler) Delete(ctx context.Context, sc scope, id string) error {
	return h.Service.Delete(ctx, appusernotifications.IDCommand{
		OrganizationID: sc.OrganizationID,
		UserID:         sc.UserID,
		ID:             id,
	})
}

func (h Handler) BatchDelete(ctx context.Context, sc scope, request BatchDeleteRequest) (int64, error) {
	return h.Service.BatchDelete(ctx, appusernotifications.BatchDeleteCommand{
		OrganizationID: sc.OrganizationID,
		UserID:         sc.UserID,
		IDs:            request.IDs,
	})
}

func (h Handler) ListReplies(ctx context.Context, sc scope, id string) ([]appusernotifications.ReplyDTO, error) {
	return h.Service.ListReplies(ctx, appusernotifications.IDCommand{
		OrganizationID: sc.OrganizationID,
		UserID:         sc.UserID,
		ID:             id,
	})
}

func (h Handler) CreateReply(ctx context.Context, sc scope, id string, request ReplyRequest) (*appusernotifications.ReplyDTO, error) {
	return h.Service.CreateReply(ctx, appusernotifications.ReplyCommand{
		OrganizationID: sc.OrganizationID,
		UserID:         sc.UserID,
		NotificationID: id,
		Content:        request.Content,
	})
}

func (h Handler) ListPreferences(ctx context.Context, sc scope) ([]appusernotifications.PreferenceDTO, error) {
	return h.Service.ListPreferences(ctx, sc.OrganizationID, sc.UserID)
}

func (h Handler) UpdatePreferences(ctx context.Context, sc scope, request PreferencesRequest) ([]appusernotifications.PreferenceDTO, error) {
	items := make([]appusernotifications.PreferenceDTO, 0, len(request.Preferences))
	for _, item := range request.Preferences {
		items = append(items, appusernotifications.PreferenceDTO{
			Category: item.Category,
			Channel:  item.Channel,
			Enabled:  item.Enabled,
		})
	}
	return h.Service.UpdatePreferences(ctx, appusernotifications.PreferencesCommand{
		OrganizationID: sc.OrganizationID,
		UserID:         sc.UserID,
		Preferences:    items,
	})
}

func (h Handler) ListChannelBindings(ctx context.Context, sc scope) ([]appusernotifications.ChannelBindingDTO, error) {
	return h.Service.ListChannelBindings(ctx, sc.OrganizationID, sc.UserID)
}

func (h Handler) UpsertChannelBinding(ctx context.Context, sc scope, request ChannelBindingRequest) (*appusernotifications.ChannelBindingDTO, error) {
	return h.Service.UpsertChannelBinding(ctx, appusernotifications.ChannelBindingCommand{
		OrganizationID: sc.OrganizationID,
		UserID:         sc.UserID,
		Channel:        request.Channel,
		ExternalID:     request.ExternalID,
	})
}

func (h Handler) DeleteChannelBinding(ctx context.Context, sc scope, id string) error {
	return h.Service.DeleteChannelBinding(ctx, appusernotifications.IDCommand{
		OrganizationID: sc.OrganizationID,
		UserID:         sc.UserID,
		ID:             id,
	})
}
