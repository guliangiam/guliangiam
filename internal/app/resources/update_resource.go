package resources

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PatchResourceCommand struct {
	ID          string
	DisplayName *string
	Type        *string
	Tag         *string
	FilePath    *string
	FileSize    *int64
	MimeType    *string
	Revision    int64
}

func (s Service) PatchResource(ctx context.Context, organizationID string, command PatchResourceCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if err := validatePatchResourceCommand(command); err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		resource, err := tx.Resources().GetByID(ctx, command.ID, organizationID)
		if err != nil {
			return err
		}

		if command.Revision > 0 && resource.Revision != command.Revision {
			return apperrors.NewConflict("resource revision mismatch")
		}

		if command.DisplayName != nil {
			resource.DisplayName = *command.DisplayName
		}
		if command.Type != nil {
			resource.Type = *command.Type
		}
		if command.Tag != nil {
			resource.Tag = *command.Tag
		}
		if command.FilePath != nil {
			resource.FilePath = *command.FilePath
		}
		if command.FileSize != nil {
			resource.FileSize = *command.FileSize
		}
		if command.MimeType != nil {
			resource.MimeType = *command.MimeType
		}

		resource.UpdatedAt = s.Clock.Now()
		resource.Revision++

		if err := tx.Resources().Update(ctx, resource); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			resource.OrganizationID,
			"resources.updated",
			resource.ID,
			"resources.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
