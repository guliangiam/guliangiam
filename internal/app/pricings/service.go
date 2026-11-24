package pricings

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/pricings"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.CommerceUoW
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.UnitOfWork) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}

type ListPricingsQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	IsEnabled               *bool
}
type CreatePricingCommand struct {
	OrganizationID, Name, DisplayName, Description, Application string
	Plans                                                       []string
	IsEnabled                                                   bool
	TrialDuration                                               int
}
type PatchPricingCommand struct {
	ID                                    string
	DisplayName, Description, Application *string
	Plans                                 []string
	IsEnabled                             *bool
	TrialDuration                         *int
	Revision                              int64
}

type PricingListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	IsEnabled      bool      `json:"isEnabled"`
	TrialDuration  int       `json:"trialDuration"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type PricingDetail struct {
	PricingListItem
	Description string   `json:"description,omitempty"`
	Plans       []string `json:"plans,omitempty"`
	Application string   `json:"application,omitempty"`
	Revision    int64    `json:"revision"`
}

func toListItem(p *domain.Pricing) PricingListItem {
	return PricingListItem{ID: p.ID, OrganizationID: p.OrganizationID, Name: p.Name, DisplayName: p.DisplayName,
		IsEnabled: p.IsEnabled, TrialDuration: p.TrialDuration, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}
func toDetail(p *domain.Pricing) PricingDetail {
	return PricingDetail{PricingListItem: toListItem(p), Description: p.Description,
		Plans: append([]string(nil), p.Plans...), Application: p.Application, Revision: p.Revision}
}

func (s Service) ListPricings(ctx context.Context, q ListPricingsQuery) ([]PricingListItem, int64, error) {
	items, total, err := s.UOW.Pricings().List(ctx, domain.ListQuery(q))
	if err != nil {
		return nil, 0, err
	}
	r := make([]PricingListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func (s Service) GetPricing(ctx context.Context, organizationID, id string) (*PricingDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	p, err := s.UOW.Pricings().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toDetail(p)
	return &d, nil
}
func (s Service) CreatePricing(ctx context.Context, c CreatePricingCommand) error {
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
	p := &domain.Pricing{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		Description: c.Description, Plans: append([]string(nil), c.Plans...), IsEnabled: c.IsEnabled,
		TrialDuration: c.TrialDuration, Application: c.Application, CreatedAt: now, UpdatedAt: now, Revision: 1}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Pricings().Create(ctx, p); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			p.OrganizationID, "",
			"pricings.create", "pricings", p.ID,
			"pricings", "pricings.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) PatchPricing(ctx context.Context, organizationID string, c PatchPricingCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		p, err := tx.Pricings().GetByID(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if c.Revision > 0 && p.Revision != c.Revision {
			return apperrors.NewConflict("pricing revision mismatch")
		}
		if c.DisplayName != nil {
			p.DisplayName = *c.DisplayName
		}
		if c.Description != nil {
			p.Description = *c.Description
		}
		if c.Application != nil {
			p.Application = *c.Application
		}
		if c.Plans != nil {
			p.Plans = append([]string(nil), c.Plans...)
		}
		if c.IsEnabled != nil {
			p.IsEnabled = *c.IsEnabled
		}
		if c.TrialDuration != nil {
			p.TrialDuration = *c.TrialDuration
		}
		p.UpdatedAt = s.Clock.Now()
		p.Revision++
		if err := tx.Pricings().Update(ctx, p); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			p.OrganizationID, "",
			"pricings.update", "pricings", p.ID,
			"pricings", "pricings.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) DeletePricing(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		p, err := tx.Pricings().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Pricings().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			p.OrganizationID, "",
			"pricings.delete", "pricings", id,
			"pricings", "pricings.delete",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
