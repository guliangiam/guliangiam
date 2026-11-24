package groups

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/groups"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type CreateGroupCommand struct {
	OrganizationID string
	// ActorID is the operator user id (empty for system actions).
	ActorID      string
	Name         string
	DisplayName  string
	Manager      string
	ContactEmail string
	Type         string
	ParentID     string
	IsTopGroup   bool
	IsEnabled    bool
	Homepage     string
	Users        []string
}

func (s *Service) CreateGroup(ctx context.Context, command CreateGroupCommand) (*groups.Group, error) {
	if err := validateCreateGroupCommand(command); err != nil {
		return nil, err
	}

	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	// 闭集裁定（G7-s）：留空回落 virtual，保证新建行永远落在合法类型集内。
	groupType := strings.TrimSpace(command.Type)
	if groupType == "" {
		groupType = groups.GroupTypeVirtual
	}
	group := &groups.Group{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Name:           command.Name,
		DisplayName:    command.DisplayName,
		Manager:        command.Manager,
		ContactEmail:   command.ContactEmail,
		Type:           groupType,
		ParentID:       command.ParentID,
		IsTopGroup:     command.IsTopGroup,
		IsEnabled:      command.IsEnabled,
		Homepage:       command.Homepage,
		Users:          append([]string(nil), command.Users...),
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	err = runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		_, err := tx.Groups().GetByID(ctx, group.ID)
		if err == nil {
			return apperrors.NewConflict("group id already exists")
		}
		if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeNotFound {
			return err
		}

		if err := validateGroupParent(ctx, tx, group.OrganizationID, group.ID, group.ParentID); err != nil {
			return err
		}
		normalized, err := normalizeGroupUsers(ctx, tx, group.OrganizationID, group.Users)
		if err != nil {
			return err
		}
		group.Users = normalized
		if err := tx.Groups().Create(ctx, group); err != nil {
			return err

		}
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			group.OrganizationID, command.ActorID,
			"groups.created", "groups", group.ID,
			"groups", "groups.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return group, nil
}
