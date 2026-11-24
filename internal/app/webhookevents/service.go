package webhookevents

import (
	"context"
	"encoding/json"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// Service exposes webhook_event use cases.
type Service struct {
	UOW appuow.AuditUoW
}

// NewService constructs a webhook_event Service.
func NewService(uow appuow.UnitOfWork) Service {
	return Service{UOW: uow}
}

// ─── Aliases ───────────────────────────────────────────────────────────────

type ListQuery = domain.ListQuery

// ─── DTOs ──────────────────────────────────────────────────────────────────

type WebhookEventDTO struct {
	ID             string          `json:"id"`
	WebhookID      string          `json:"webhookId"`
	EventType      string          `json:"eventType"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	ResponseCode   int             `json:"responseCode,omitempty"`
	ResponseBody   string          `json:"responseBody,omitempty"`
	Status         string          `json:"status"`
	RetryCount     int             `json:"retryCount"`
	NextRetryAt    *time.Time      `json:"nextRetryAt,omitempty"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

func toDTO(e *domain.WebhookEvent) WebhookEventDTO {
	return WebhookEventDTO{
		ID:             e.ID,
		WebhookID:      e.WebhookID,
		EventType:      e.EventType,
		Payload:        e.Payload,
		ResponseCode:   e.ResponseCode,
		ResponseBody:   e.ResponseBody,
		Status:         e.Status,
		RetryCount:     e.RetryCount,
		NextRetryAt:    e.NextRetryAt,
		IdempotencyKey: e.IdempotencyKey,
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
	}
}

// ─── Use cases ────────────────────────────────────────────────────────────

func (s Service) List(ctx context.Context, q ListQuery) ([]WebhookEventDTO, int64, error) {
	if err := ensureListTenantScope(ctx, q.OrganizationID); err != nil {
		return nil, 0, err
	}
	repo := s.UOW.WebhookEvents()
	items, total, err := repo.List(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]WebhookEventDTO, 0, len(items))
	for _, e := range items {
		dtos = append(dtos, toDTO(e))
	}
	return dtos, total, nil
}

// ensureListTenantScope binds the cross-webhook audit list to a tenant.
//
// webhook_events carries no organization_id column (and no RLS): its rows
// inherit the tenant of their parent webhook, which the repository enforces
// with an EXISTS predicate on guliangiam.webhooks. A non-empty organizationID
// is therefore required for regular callers; an empty one is only meaningful
// for an authenticated platform scope (super_admin/system, resolved by
// transport), which keeps the existing all-tenant audit view.
//
// Without this guard an empty tenant would silently degrade to a global read,
// which on MySQL (no RLS fallback) is a cross-tenant leak. The error code
// matches the transport guard so callers always see one 400, never a 404/500
// depending on the layer.
func ensureListTenantScope(ctx context.Context, organizationID string) error {
	if organizationID != "" {
		return nil
	}
	scope := tenantctx.TenantScopeFromContext(ctx)
	if scope.Mode == tenantctx.TenantFilterSuperAdmin || scope.Mode == tenantctx.TenantFilterSystem {
		return nil
	}
	return apperrors.NewInvalidArgument("organizationId is required")
}
