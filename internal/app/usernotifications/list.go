package usernotifications

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type ListQuery struct {
	OrganizationID string
	UserID         string
	Category       string
	IsRead         *bool
	Keyword        string
	Page           int
	PageSize       int
}

func (s Service) List(ctx context.Context, query ListQuery) (*NotificationListDTO, error) {
	if query.OrganizationID == "" || query.UserID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId and userId are required")
	}
	if query.Category != "" && !validCategories[query.Category] {
		return nil, apperrors.NewInvalidArgument("invalid notification category")
	}
	page, pageSize := normalizePagination(query.Page, query.PageSize)

	var items []*domain.Notification
	var total int64
	err := withInbox(ctx, s.UOW, func(ctx context.Context, repo domain.Repository) error {
		var listErr error
		items, total, listErr = repo.List(ctx, query.OrganizationID, query.UserID, domain.ListFilter{
			Category: query.Category,
			IsRead:   query.IsRead,
			Keyword:  query.Keyword,
			Page:     page,
			PageSize: pageSize,
		})
		return listErr
	})
	if err != nil {
		return nil, err
	}

	result := &NotificationListDTO{
		Items:    make([]NotificationDTO, 0, len(items)),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	for _, item := range items {
		result.Items = append(result.Items, toNotificationDTO(item))
	}
	return result, nil
}

func (s Service) UnreadCount(ctx context.Context, organizationID, userID string) (int64, error) {
	if organizationID == "" || userID == "" {
		return 0, apperrors.NewInvalidArgument("organizationId and userId are required")
	}
	return withInboxCount(ctx, s.UOW, organizationID, userID)

}

func withInboxCount(ctx context.Context, unitOfWork appuow.UserNotificationsUoW, organizationID, userID string) (int64, error) {
	var total int64
	err := withInbox(ctx, unitOfWork, func(ctx context.Context, repo domain.Repository) error {
		var countErr error
		total, countErr = repo.UnreadCount(ctx, organizationID, userID)
		return countErr
	})
	return total, err
}

func normalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
