package models

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/models"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type ListModelsQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type CreateModelCommand struct {
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	ModelText      string
	// Phase 2 fields
	Owner     string
	IsEnabled bool
}

type PatchModelCommand struct {
	ID          string
	DisplayName *string
	Description *string
	ModelText   *string
	Revision    int64
	// Phase 2 fields
	Owner     *string
	IsEnabled *bool
}

func (s Service) ListModels(ctx context.Context, query ListModelsQuery) ([]ModelListItem, int64, error) {
	models, total, err := s.UOW.Models().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}
	items := make([]ModelListItem, 0, len(models))
	for _, m := range models {
		items = append(items, toListItem(m))
	}
	return items, total, nil
}

func (s Service) GetModel(ctx context.Context, organizationID, id string) (*ModelDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	m, err := s.UOW.Models().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	detail := toDetail(m)
	return &detail, nil
}

func (s Service) CreateModel(ctx context.Context, command CreateModelCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}

	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()

	model := &domain.Model{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Name:           command.Name,
		DisplayName:    command.DisplayName,
		Description:    command.Description,
		ModelText:      command.ModelText,
		Owner:          command.Owner,
		IsEnabled:      command.IsEnabled,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Models().Create(ctx, model); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			model.OrganizationID, "",
			"models.create", "models", model.ID,
			"models", "models.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

func (s Service) PatchModel(ctx context.Context, organizationID string, command PatchModelCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		model, err := tx.Models().GetByIDForOrganization(ctx, command.ID, organizationID)
		if err != nil {
			return err
		}
		if command.Revision > 0 && model.Revision != command.Revision {
			return apperrors.NewConflict("model revision mismatch")
		}
		if command.DisplayName != nil {
			model.DisplayName = *command.DisplayName
		}
		if command.Description != nil {
			model.Description = *command.Description
		}
		if command.ModelText != nil {
			model.ModelText = *command.ModelText
		}
		// Phase 2 field patching
		if command.Owner != nil {
			model.Owner = *command.Owner
		}
		if command.IsEnabled != nil {
			model.IsEnabled = *command.IsEnabled
		}
		model.UpdatedAt = s.Clock.Now()
		model.Revision++
		if err := tx.Models().Update(ctx, model); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			model.OrganizationID, "",
			"models.update", "models", model.ID,
			"models", "models.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

func (s Service) DeleteModel(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		model, err := tx.Models().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Models().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			model.OrganizationID, "",
			"models.delete", "models", id,
			"models", "models.delete",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
