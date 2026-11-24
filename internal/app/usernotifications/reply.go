package usernotifications

import (
	"context"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type ReplyCommand struct {
	OrganizationID string
	UserID         string
	NotificationID string
	Content        string
}

func (s Service) ListReplies(ctx context.Context, command IDCommand) ([]ReplyDTO, error) {
	if err := requireScope(command.OrganizationID, command.UserID); err != nil {
		return nil, err
	}
	if command.ID == "" {
		return nil, apperrors.NewInvalidArgument("notification id is required")
	}
	var items []*domain.Reply
	err := withInbox(ctx, s.UOW, func(ctx context.Context, repo domain.Repository) error {
		if _, getErr := repo.GetForUser(ctx, command.ID, command.OrganizationID, command.UserID); getErr != nil {
			return getErr
		}
		var listErr error
		items, listErr = repo.ListReplies(ctx, command.ID, command.OrganizationID, command.UserID)
		return listErr
	})
	if err != nil {
		return nil, err
	}
	result := make([]ReplyDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toReplyDTO(item))
	}
	return result, nil
}

func (s Service) CreateReply(ctx context.Context, command ReplyCommand) (*ReplyDTO, error) {
	if err := requireScope(command.OrganizationID, command.UserID); err != nil {
		return nil, err
	}
	if command.NotificationID == "" {
		return nil, apperrors.NewInvalidArgument("notification id is required")
	}
	content := strings.TrimSpace(command.Content)
	if content == "" {
		return nil, apperrors.NewInvalidArgument("reply content is required")
	}
	if len([]rune(content)) > 4000 {
		return nil, apperrors.NewInvalidArgument("reply content is too long")
	}

	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate reply id")
	}
	now := s.Clock.Now()
	reply := &domain.Reply{
		ID:             id,
		NotificationID: command.NotificationID,
		OrganizationID: command.OrganizationID,
		UserID:         command.UserID,
		Content:        content,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	err = withInbox(ctx, s.UOW, func(ctx context.Context, repo domain.Repository) error {
		notification, getErr := repo.GetForUser(ctx, command.NotificationID, command.OrganizationID, command.UserID)
		if getErr != nil {
			return getErr
		}
		if !notification.AllowReply {
			return apperrors.NewInvalidArgument("this notification does not accept replies")
		}
		return repo.CreateReply(ctx, reply)
	})
	if err != nil {
		return nil, err
	}
	result := toReplyDTO(reply)
	return &result, nil
}
