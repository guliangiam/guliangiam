package resources

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/resources"
)

type CreateResourceCommand struct {
	OrganizationID string
	ApplicationID  string
	Name           string
	DisplayName    string
	Type           string
	Tag            string
	FilePath       string
	FileSize       int64
	MimeType       string
}

func (s Service) NewResource(command CreateResourceCommand) (*domain.Resource, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()

	return &domain.Resource{
		ID:             id,
		OrganizationID: command.OrganizationID,
		ApplicationID:  command.ApplicationID,
		Name:           command.Name,
		DisplayName:    command.DisplayName,
		Type:           command.Type,
		Tag:            command.Tag,
		FilePath:       command.FilePath,
		FileSize:       command.FileSize,
		MimeType:       command.MimeType,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}, nil
}

func (s Service) CreateResource(ctx context.Context, command CreateResourceCommand) error {
	if err := validateCreateResourceCommand(command); err != nil {
		return err
	}

	// command.OrganizationID 直接来自 JSON body（resources chi.go 未做 TrustedOrganizationID 收敛，行为缺口）；写入按 body 值落 organization_id，越权由调用方作用域与 RLS WITH CHECK 拦截。
	resource, err := s.NewResource(command)
	if err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Resources().Create(ctx, resource); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			resource.OrganizationID,
			"resources.created",
			resource.ID,
			"resources.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
