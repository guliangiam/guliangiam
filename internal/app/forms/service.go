package forms

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/forms"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.InfraUoW
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.InfraUoW) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}

type ListFormsQuery struct {
	Page, PageSize                int
	OrganizationID, Keyword, Type string
	IsEnabled                     *bool
}
type CreateFormCommand struct {
	OrganizationID, Name, DisplayName, Description, Type, Properties string
	IsEnabled                                                        bool
}
type PatchFormCommand struct {
	ID                                         string
	DisplayName, Description, Type, Properties *string
	IsEnabled                                  *bool
	Revision                                   int64
}

type FormListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Type           string    `json:"type,omitempty"`
	IsEnabled      bool      `json:"isEnabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type FormDetail struct {
	FormListItem
	Description string `json:"description,omitempty"`
	Properties  string `json:"properties,omitempty"`
	Revision    int64  `json:"revision"`
}

func toListItem(f *domain.Form) FormListItem {
	return FormListItem{ID: f.ID, OrganizationID: f.OrganizationID, Name: f.Name, DisplayName: f.DisplayName,
		Type: f.Type, IsEnabled: f.IsEnabled, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt}
}
func toDetail(f *domain.Form) FormDetail {
	return FormDetail{FormListItem: toListItem(f), Description: f.Description, Properties: f.Properties, Revision: f.Revision}
}

func (s Service) ListForms(ctx context.Context, q ListFormsQuery) ([]FormListItem, int64, error) {
	items, total, err := s.UOW.Forms().List(ctx, domain.ListQuery{Page: q.Page, PageSize: q.PageSize, OrganizationID: q.OrganizationID, Keyword: q.Keyword, Type: q.Type, IsEnabled: q.IsEnabled})
	if err != nil {
		return nil, 0, err
	}
	r := make([]FormListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func (s Service) GetForm(ctx context.Context, organizationID, id string) (*FormDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	f, err := s.UOW.Forms().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toDetail(f)
	return &d, nil
}
func (s Service) CreateForm(ctx context.Context, c CreateFormCommand) error {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	f := &domain.Form{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		Description: c.Description, Type: c.Type, Properties: c.Properties, IsEnabled: c.IsEnabled,
		CreatedAt: now, UpdatedAt: now, Revision: 1}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Forms().Create(ctx, f); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			f.OrganizationID, "",
			"forms.create", "forms", f.ID,
			"forms", "forms.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) PatchForm(ctx context.Context, organizationID string, c PatchFormCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		f, err := tx.Forms().GetByID(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if c.Revision > 0 && f.Revision != c.Revision {
			return apperrors.NewConflict("form revision mismatch")
		}
		if c.DisplayName != nil {
			f.DisplayName = *c.DisplayName
		}
		if c.Description != nil {
			f.Description = *c.Description
		}
		if c.Type != nil {
			f.Type = *c.Type
		}
		if c.Properties != nil {
			f.Properties = *c.Properties
		}
		if c.IsEnabled != nil {
			f.IsEnabled = *c.IsEnabled
		}
		f.UpdatedAt = s.Clock.Now()
		f.Revision++
		if err := tx.Forms().Update(ctx, f); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			f.OrganizationID, "",
			"forms.update", "forms", f.ID,
			"forms", "forms.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) DeleteForm(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		f, err := tx.Forms().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Forms().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			f.OrganizationID, "",
			"forms.delete", "forms", id,
			"forms", "forms.delete",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
