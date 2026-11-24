package usernotifications

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type IDCommand struct {
	OrganizationID string
	UserID         string
	ID             string
}

func (s Service) Get(ctx context.Context, command IDCommand) (*NotificationDTO, error) {
	if err := requireScope(command.OrganizationID, command.UserID); err != nil {
		return nil, err
	}
	if command.ID == "" {
		return nil, apperrors.NewInvalidArgument("notification id is required")
	}
	var item *domain.Notification
	err := withInbox(ctx, s.UOW, func(ctx context.Context, repo domain.Repository) error {
		var getErr error
		item, getErr = repo.GetForUser(ctx, command.ID, command.OrganizationID, command.UserID)
		return getErr
	})
	if err != nil {
		return nil, err
	}
	result := toNotificationDTO(item)
	return &result, nil
}

type MarkReadCommand struct {
	OrganizationID string
	UserID         string
	ID             string
	Read           bool
}

func (s Service) MarkRead(ctx context.Context, command MarkReadCommand) error {
	if err := requireScope(command.OrganizationID, command.UserID); err != nil {
		return err
	}
	if command.ID == "" {
		return apperrors.NewInvalidArgument("notification id is required")
	}
	now := s.Clock.Now()
	return withInbox(ctx, s.UOW, func(ctx context.Context, repo domain.Repository) error {
		return repo.MarkRead(ctx, command.ID, command.OrganizationID, command.UserID, command.Read, now)
	})
}

type MarkAllReadCommand struct {
	OrganizationID string
	UserID         string
	Category       string
}

func (s Service) MarkAllRead(ctx context.Context, command MarkAllReadCommand) (int64, error) {
	if err := requireScope(command.OrganizationID, command.UserID); err != nil {
		return 0, err
	}
	if command.Category != "" && !validCategories[command.Category] {
		return 0, apperrors.NewInvalidArgument("invalid notification category")
	}
	now := s.Clock.Now()
	var affected int64
	err := withInbox(ctx, s.UOW, func(ctx context.Context, repo domain.Repository) error {
		var markErr error
		affected, markErr = repo.MarkAllRead(ctx, command.OrganizationID, command.UserID, command.Category, now)
		return markErr
	})
	return affected, err
}

func (s Service) Delete(ctx context.Context, command IDCommand) error {
	if err := requireScope(command.OrganizationID, command.UserID); err != nil {
		return err
	}
	if command.ID == "" {
		return apperrors.NewInvalidArgument("notification id is required")
	}
	now := s.Clock.Now()
	return withInbox(ctx, s.UOW, func(ctx context.Context, repo domain.Repository) error {
		return repo.SoftDelete(ctx, command.ID, command.OrganizationID, command.UserID, now)
	})
}

type BatchDeleteCommand struct {
	OrganizationID string
	UserID         string
	IDs            []string
}

func (s Service) BatchDelete(ctx context.Context, command BatchDeleteCommand) (int64, error) {
	if err := requireScope(command.OrganizationID, command.UserID); err != nil {
		return 0, err
	}
	if len(command.IDs) == 0 {
		return 0, apperrors.NewInvalidArgument("notification ids are required")
	}
	now := s.Clock.Now()
	var affected int64
	err := withInbox(ctx, s.UOW, func(ctx context.Context, repo domain.Repository) error {
		var deleteErr error
		affected, deleteErr = repo.BatchSoftDelete(ctx, command.IDs, command.OrganizationID, command.UserID, now)
		return deleteErr
	})
	return affected, err
}

func requireScope(organizationID, userID string) error {
	if organizationID == "" || userID == "" {
		return apperrors.NewInvalidArgument("organizationId and userId are required")
	}
	return nil
}
