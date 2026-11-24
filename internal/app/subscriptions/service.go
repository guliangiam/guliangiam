package subscriptions

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/subscriptions"
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

type ListSubscriptionsQuery struct {
	Page, PageSize                int
	OrganizationID, Keyword, User string
	State                         *string
}
type CreateSubscriptionCommand struct {
	OrganizationID, Name, DisplayName, User, Plan, Pricing, State string
	StartAt, EndAt, ExpireAt                                      *time.Time
}
type PatchSubscriptionCommand struct {
	ID                       string
	DisplayName, State       *string
	StartAt, EndAt, ExpireAt *time.Time
	Revision                 int64
}

type SubscriptionListItem struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Name           string     `json:"name"`
	DisplayName    string     `json:"displayName"`
	User           string     `json:"user,omitempty"`
	Plan           string     `json:"plan,omitempty"`
	State          string     `json:"state"`
	StartAt        *time.Time `json:"startAt,omitempty"`
	EndAt          *time.Time `json:"endAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}
type SubscriptionDetail struct {
	SubscriptionListItem
	Pricing  string     `json:"pricing,omitempty"`
	ExpireAt *time.Time `json:"expireAt,omitempty"`
	Revision int64      `json:"revision"`
}

func toListItem(s *domain.Subscription) SubscriptionListItem {
	return SubscriptionListItem{ID: s.ID, OrganizationID: s.OrganizationID, Name: s.Name, DisplayName: s.DisplayName,
		User: s.User, Plan: s.Plan, State: s.State, StartAt: s.StartAt, EndAt: s.EndAt,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
}
func toDetail(s *domain.Subscription) SubscriptionDetail {
	return SubscriptionDetail{SubscriptionListItem: toListItem(s), Pricing: s.Pricing, ExpireAt: s.ExpireAt, Revision: s.Revision}
}

func (svc Service) ListSubscriptions(ctx context.Context, q ListSubscriptionsQuery) ([]SubscriptionListItem, int64, error) {
	items, total, err := svc.UOW.Subscriptions().List(ctx, domain.ListQuery(q))
	if err != nil {
		return nil, 0, err
	}
	r := make([]SubscriptionListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func (svc Service) GetSubscription(ctx context.Context, organizationID, id string) (*SubscriptionDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	s, err := svc.UOW.Subscriptions().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	if err := ensureSubscriptionTenantScope(s, organizationID); err != nil {
		return nil, err
	}
	d := toDetail(s)
	return &d, nil
}
func (svc Service) CreateSubscription(ctx context.Context, c CreateSubscriptionCommand) error {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	id, err := svc.UUID.NewString()
	if err != nil {
		return err
	}
	now := svc.Clock.Now()
	state := domain.StateActive
	if c.State != "" {
		state = c.State
	}
	s := &domain.Subscription{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		User: c.User, Plan: c.Plan, Pricing: c.Pricing, State: state,
		StartAt: c.StartAt, EndAt: c.EndAt, ExpireAt: c.ExpireAt,
		CreatedAt: now, UpdatedAt: now, Revision: 1}
	if err := s.Validate(); err != nil {
		return err
	}
	return svc.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := svc.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			s.OrganizationID, s.User,
			"subscription.created", "Subscription", s.ID,
			"Subscription", "subscription.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return tx.Subscriptions().Create(ctx, s)
	})
}
func (svc Service) PatchSubscription(ctx context.Context, organizationID string, c PatchSubscriptionCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return svc.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation, so a cross-tenant request
		// leaves no write and no audit record behind.
		s, err := tx.Subscriptions().GetByIDForOrganization(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if err := ensureSubscriptionTenantScope(s, organizationID); err != nil {
			return err
		}
		if c.Revision > 0 && s.Revision != c.Revision {
			return apperrors.NewConflict("subscription revision mismatch")
		}
		if c.DisplayName != nil {
			s.DisplayName = *c.DisplayName
		}
		if c.State != nil {
			if err := s.TransitionTo(*c.State); err != nil {
				return err
			}
		}
		if c.StartAt != nil {
			s.StartAt = c.StartAt
		}
		if c.EndAt != nil {
			s.EndAt = c.EndAt
		}
		if c.ExpireAt != nil {
			s.ExpireAt = c.ExpireAt
		}
		// PATCH 合并后的时间窗必须整体重跑领域不变量（start < end < expire），
		// 否则 endAt 早于 startAt 的订阅会绕过 Create 路径的校验落库（BE-C-14）。
		if err := s.Validate(); err != nil {
			return err
		}
		s.UpdatedAt = svc.Clock.Now()
		s.Revision++
		if err := svc.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			s.OrganizationID, s.User,
			"subscription.updated", "Subscription", s.ID,
			"Subscription", "subscription.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return tx.Subscriptions().Update(ctx, s)
	})
}
func (svc Service) DeleteSubscription(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return svc.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the audit
		// row carries the entity's real organization.
		s, err := tx.Subscriptions().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := ensureSubscriptionTenantScope(s, organizationID); err != nil {
			return err
		}
		if err := svc.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			s.OrganizationID, s.User,
			"subscription.deleted", "Subscription", s.ID,
			"Subscription", "subscription.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return tx.Subscriptions().Delete(ctx, id, organizationID)
	})
}
