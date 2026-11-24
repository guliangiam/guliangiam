package plans

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/plans"
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

type ListPlansQuery struct {
	Page, PageSize                  int
	OrganizationID, Keyword, Period string
	State                           string
	IsExclusive                     *bool
}
type CreatePlanCommand struct {
	OrganizationID, Name, DisplayName, Description, Currency, Period, Product, Role string
	Price                                                                           float64
	PaymentProviders                                                                []string
	State                                                                           string
	IsExclusive                                                                     bool
}
type PatchPlanCommand struct {
	ID                                                        string
	DisplayName, Description, Currency, Period, Product, Role *string
	Price                                                     *float64
	PaymentProviders                                          []string
	State                                                     *string
	IsExclusive                                               *bool
	Revision                                                  int64
}

type PlanListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Price          float64   `json:"price"`
	Currency       string    `json:"currency,omitempty"`
	Period         string    `json:"period,omitempty"`
	State          string    `json:"state"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type PlanDetail struct {
	PlanListItem
	Description      string   `json:"description,omitempty"`
	Product          string   `json:"product,omitempty"`
	PaymentProviders []string `json:"paymentProviders,omitempty"`
	IsExclusive      bool     `json:"isExclusive"`
	Role             string   `json:"role,omitempty"`
	Revision         int64    `json:"revision"`
}

func toListItem(p *domain.Plan) PlanListItem {
	return PlanListItem{ID: p.ID, OrganizationID: p.OrganizationID, Name: p.Name, DisplayName: p.DisplayName,
		Price: p.Price, Currency: p.Currency, Period: p.Period, State: p.State,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}
func toDetail(p *domain.Plan) PlanDetail {
	return PlanDetail{PlanListItem: toListItem(p), Description: p.Description, Product: p.Product,
		PaymentProviders: append([]string(nil), p.PaymentProviders...), IsExclusive: p.IsExclusive,
		Role: p.Role, Revision: p.Revision}
}

func (s Service) ListPlans(ctx context.Context, q ListPlansQuery) ([]PlanListItem, int64, error) {
	items, total, err := s.UOW.Plans().List(ctx, domain.ListQuery{Page: q.Page, PageSize: q.PageSize, OrganizationID: q.OrganizationID, Keyword: q.Keyword, State: q.State, Period: q.Period, IsExclusive: q.IsExclusive})
	if err != nil {
		return nil, 0, err
	}
	r := make([]PlanListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func (s Service) GetPlan(ctx context.Context, organizationID, id string) (*PlanDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	p, err := s.UOW.Plans().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toDetail(p)
	return &d, nil
}
func (s Service) CreatePlan(ctx context.Context, c CreatePlanCommand) error {
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
	p := &domain.Plan{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		Description: c.Description, Price: c.Price, Currency: c.Currency, Period: c.Period,
		Product: c.Product, PaymentProviders: append([]string(nil), c.PaymentProviders...),
		State: c.State, IsExclusive: c.IsExclusive, Role: c.Role,
		CreatedAt: now, UpdatedAt: now, Revision: 1}
	if err := p.Validate(); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Plans().Create(ctx, p); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			p.OrganizationID, "",
			"plans.create", "plans", p.ID,
			"plans", "plans.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) PatchPlan(ctx context.Context, organizationID string, c PatchPlanCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		p, err := tx.Plans().GetByID(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if c.Revision > 0 && p.Revision != c.Revision {
			return apperrors.NewConflict("plan revision mismatch")
		}
		if c.DisplayName != nil {
			p.DisplayName = *c.DisplayName
		}
		if c.Description != nil {
			p.Description = *c.Description
		}
		if c.Currency != nil {
			p.Currency = *c.Currency
		}
		if c.Period != nil {
			p.Period = *c.Period
		}
		if c.Product != nil {
			p.Product = *c.Product
		}
		if c.Role != nil {
			p.Role = *c.Role
		}
		if c.Price != nil {
			p.Price = *c.Price
		}
		if c.PaymentProviders != nil {
			p.PaymentProviders = append([]string(nil), c.PaymentProviders...)
		}
		if c.State != nil {
			if err := p.TransitionTo(*c.State); err != nil {
				return err
			}
		}
		if c.IsExclusive != nil {
			p.IsExclusive = *c.IsExclusive
		}
		p.UpdatedAt = s.Clock.Now()
		p.Revision++
		if err := tx.Plans().Update(ctx, p); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			p.OrganizationID, "",
			"plans.update", "plans", p.ID,
			"plans", "plans.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) DeletePlan(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		p, err := tx.Plans().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Plans().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			p.OrganizationID, "",
			"plans.delete", "plans", id,
			"plans", "plans.delete",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
