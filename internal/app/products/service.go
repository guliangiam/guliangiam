package products

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/products"
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

type ListProductsQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	State                   *string
	IsRecharge              *bool
}
type CreateProductCommand struct {
	OrganizationID, Name, DisplayName, Image, Detail, Description, Tag, Currency string
	Price                                                                        float64
	Quantity, Sold                                                               int
	IsRecharge                                                                   bool
	RechargeOptions                                                              []float64
	DisableCustomRecharge                                                        bool
	Providers                                                                    []string
	SuccessUrl, State                                                            string
}
type PatchProductCommand struct {
	ID                                                     string
	DisplayName, Image, Detail, Description, Tag, Currency *string
	Price                                                  *float64
	Quantity, Sold                                         *int
	IsRecharge, DisableCustomRecharge                      *bool
	RechargeOptions                                        []float64
	Providers                                              []string
	SuccessUrl, State                                      *string
	Revision                                               int64
}

type ProductListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Image          string    `json:"image,omitempty"`
	Price          float64   `json:"price"`
	Currency       string    `json:"currency,omitempty"`
	State          string    `json:"state"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type ProductDetail struct {
	ProductListItem
	Detail                string    `json:"detail,omitempty"`
	Description           string    `json:"description,omitempty"`
	Tag                   string    `json:"tag,omitempty"`
	Quantity              int       `json:"quantity"`
	Sold                  int       `json:"sold"`
	IsRecharge            bool      `json:"isRecharge"`
	RechargeOptions       []float64 `json:"rechargeOptions,omitempty"`
	DisableCustomRecharge bool      `json:"disableCustomRecharge"`
	Providers             []string  `json:"providers,omitempty"`
	SuccessUrl            string    `json:"successUrl,omitempty"`
	Revision              int64     `json:"revision"`
}

func toListItem(p *domain.Product) ProductListItem {
	return ProductListItem{ID: p.ID, OrganizationID: p.OrganizationID, Name: p.Name, DisplayName: p.DisplayName,
		Image: p.Image, Price: p.Price, Currency: p.Currency, State: p.State,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}
func toDetail(p *domain.Product) ProductDetail {
	return ProductDetail{ProductListItem: toListItem(p), Detail: p.Detail, Description: p.Description,
		Tag: p.Tag, Quantity: p.Quantity, Sold: p.Sold, IsRecharge: p.IsRecharge,
		RechargeOptions:       append([]float64(nil), p.RechargeOptions...),
		DisableCustomRecharge: p.DisableCustomRecharge,
		Providers:             append([]string(nil), p.Providers...), SuccessUrl: p.SuccessUrl, Revision: p.Revision}
}

func (s Service) ListProducts(ctx context.Context, q ListProductsQuery) ([]ProductListItem, int64, error) {
	items, total, err := s.UOW.Products().List(ctx, domain.ListQuery(q))
	if err != nil {
		return nil, 0, err
	}
	r := make([]ProductListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func (s Service) GetProduct(ctx context.Context, organizationID, id string) (*ProductDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	p, err := s.UOW.Products().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toDetail(p)
	return &d, nil
}
func (s Service) CreateProduct(ctx context.Context, c CreateProductCommand) error {
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
	state := domain.StateDraft
	if c.State != "" {
		state = c.State
	}
	p := &domain.Product{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		Image: c.Image, Detail: c.Detail, Description: c.Description, Tag: c.Tag, Currency: c.Currency,
		Price: c.Price, Quantity: c.Quantity, Sold: c.Sold, IsRecharge: c.IsRecharge,
		RechargeOptions: append([]float64(nil), c.RechargeOptions...), DisableCustomRecharge: c.DisableCustomRecharge,
		Providers: append([]string(nil), c.Providers...), SuccessUrl: c.SuccessUrl, State: state,
		CreatedAt: now, UpdatedAt: now, Revision: 1}
	if err := p.Validate(); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Products().Create(ctx, p); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			p.OrganizationID, "",
			"products.create", "products", p.ID,
			"products", "products.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) PatchProduct(ctx context.Context, organizationID string, c PatchProductCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		p, err := tx.Products().GetByIDForOrganization(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if c.Revision > 0 && p.Revision != c.Revision {
			return apperrors.NewConflict("product revision mismatch")
		}
		if c.DisplayName != nil {
			p.DisplayName = *c.DisplayName
		}
		if c.Image != nil {
			p.Image = *c.Image
		}
		if c.Detail != nil {
			p.Detail = *c.Detail
		}
		if c.Description != nil {
			p.Description = *c.Description
		}
		if c.Tag != nil {
			p.Tag = *c.Tag
		}
		if c.Currency != nil {
			p.Currency = *c.Currency
		}
		if c.Price != nil {
			p.Price = *c.Price
		}
		if c.Quantity != nil {
			p.Quantity = *c.Quantity
		}
		if c.Sold != nil {
			p.Sold = *c.Sold
		}
		if c.IsRecharge != nil {
			p.IsRecharge = *c.IsRecharge
		}
		if c.DisableCustomRecharge != nil {
			p.DisableCustomRecharge = *c.DisableCustomRecharge
		}
		if c.RechargeOptions != nil {
			p.RechargeOptions = append([]float64(nil), c.RechargeOptions...)
		}
		if c.Providers != nil {
			p.Providers = append([]string(nil), c.Providers...)
		}
		if c.SuccessUrl != nil {
			p.SuccessUrl = *c.SuccessUrl
		}
		if c.State != nil {
			if err := p.TransitionTo(*c.State); err != nil {
				return err
			}
		}
		// PATCH 合并后的聚合必须整体重跑领域不变量（Price>=0、Quantity>=0、
		// 充值选项约束），否则负价/负库存会绕过 Create 路径的校验落库（BE-C-14）。
		if err := p.Validate(); err != nil {
			return err
		}
		p.UpdatedAt = s.Clock.Now()
		p.Revision++
		if err := tx.Products().Update(ctx, p); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			p.OrganizationID, "",
			"products.update", "products", p.ID,
			"products", "products.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) DeleteProduct(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		p, err := tx.Products().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Products().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			p.OrganizationID, "",
			"products.delete", "products", id,
			"products", "products.delete",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

// BuyProduct validates product state and returns order creation info for
// checkout inside the trusted tenant.
//
// 这里的库存检查只是咨询性预检（BE-C-11）：真正的硬守卫是 PlaceOrder
// 事务内的原子扣减（sold = sold + q WHERE sold + q <= quantity），
// 并发下单的超卖由那条 SQL 拦截，而不是这里的先读后判。
func (s Service) BuyProduct(ctx context.Context, organizationID, id, user, pricingName, planName string, quantity int) (*ProductDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	p, err := s.UOW.Products().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	// Sellability has exactly one definition (§1.5): `active`. The earlier
	// `Active`/`Published` spellings were pre-registry values that no writer
	// produced, so the check rejected every product that actually existed.
	if p.State != domain.StateActive {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "product is not available for purchase", map[string]any{"field": "state", "value": string(p.State), "expected": string(domain.StateActive)})
	}
	if p.Quantity > 0 && p.Sold+quantity > p.Quantity {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "insufficient product quantity", map[string]any{"field": "quantity", "value": quantity, "max": p.Quantity - p.Sold})
	}
	d := toDetail(p)
	return &d, nil
}
