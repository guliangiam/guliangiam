package webhookevents

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestToDTOMapsPayloadAndRetryState(t *testing.T) {
	next := time.Date(2026, 10, 29, 15, 0, 0, 0, time.UTC)
	now := time.Date(2026, 10, 29, 14, 0, 0, 0, time.UTC)
	e := &domain.WebhookEvent{
		ID:             "evt-1",
		WebhookID:      "wh-1",
		EventType:      "user.created",
		Payload:        json.RawMessage(`{"id":"u1"}`),
		ResponseCode:   503,
		ResponseBody:   "unavailable",
		Status:         "retrying",
		RetryCount:     2,
		NextRetryAt:    &next,
		IdempotencyKey: "idem-1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	dto := toDTO(e)
	if dto.ID != e.ID || dto.WebhookID != e.WebhookID || dto.EventType != e.EventType {
		t.Fatalf("identity fields lost: %+v", dto)
	}
	if string(dto.Payload) != string(e.Payload) || dto.ResponseCode != 503 || dto.RetryCount != 2 {
		t.Fatalf("payload/retry lost: %+v", dto)
	}
	if dto.NextRetryAt == nil || !dto.NextRetryAt.Equal(next) || dto.IdempotencyKey != "idem-1" {
		t.Fatalf("retry schedule lost: %+v", dto)
	}
}

// ─── Tenant binding of the cross-webhook audit list ────────────────────────

const (
	auditTenantOrgID   = "11111111-1111-1111-1111-111111111111"
	auditTenantEventID = "33333333-3333-3333-3333-333333333333"
)

// auditRepo records the tenant every audit-list query receives.
type auditRepo struct {
	queries []domain.ListQuery
	items   []*domain.WebhookEvent
	total   int64
}

func (r *auditRepo) GetByID(context.Context, string) (*domain.WebhookEvent, error) {
	return nil, apperrors.NewNotFound("webhook event not found")
}

func (r *auditRepo) GetByIDForOrganization(context.Context, string, string) (*domain.WebhookEvent, error) {
	return nil, apperrors.NewNotFound("webhook event not found")
}

func (r *auditRepo) List(_ context.Context, q domain.ListQuery) ([]*domain.WebhookEvent, int64, error) {
	r.queries = append(r.queries, q)
	return r.items, r.total, nil
}

func (r *auditRepo) ListExistingIDs(context.Context, []string) ([]string, error) { return nil, nil }
func (r *auditRepo) CreateBatch(context.Context, []*domain.WebhookEvent) error   { return nil }
func (r *auditRepo) Create(context.Context, *domain.WebhookEvent) error          { return nil }
func (r *auditRepo) Update(context.Context, *domain.WebhookEvent) error          { return nil }
func (r *auditRepo) Delete(context.Context, string) error                        { return nil }

func newAuditService(t *testing.T, repo domain.Repository) Service {
	t.Helper()
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuditUoW(ctrl)
	uow.EXPECT().WebhookEvents().Return(repo).AnyTimes()
	return Service{UOW: uow}
}

func platformScope(mode tenantctx.TenantFilterMode) context.Context {
	return tenantctx.WithTenantScope(context.Background(), "", "", mode)
}

func assertInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
	assert.Equal(t, "organizationId is required", appErr.Message)
}

// TestList_EmptyTenantWithoutPlatformScopeIsRejected pins the fail-fast at the
// use-case layer: webhook_events has no organization_id column, so an empty
// tenant would degrade to a cross-tenant audit read (and MySQL has no RLS to
// compensate).
func TestList_EmptyTenantWithoutPlatformScopeIsRejected(t *testing.T) {
	repo := &auditRepo{}
	_, _, err := newAuditService(t, repo).List(context.Background(), ListQuery{Page: 1, PageSize: 10})
	assertInvalidArgument(t, err)
	assert.Empty(t, repo.queries)
}

// TestList_PlatformScopeKeepsAllTenants pins the retained platform semantics:
// super_admin/system without an explicit tenant keep the all-tenant audit view.
func TestList_PlatformScopeKeepsAllTenants(t *testing.T) {
	modes := []tenantctx.TenantFilterMode{
		tenantctx.TenantFilterSuperAdmin,
		tenantctx.TenantFilterSystem,
	}
	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			repo := &auditRepo{}
			_, _, err := newAuditService(t, repo).List(platformScope(mode), ListQuery{Page: 1, PageSize: 10})
			require.NoError(t, err)
			require.Len(t, repo.queries, 1)
			assert.Equal(t, "", repo.queries[0].OrganizationID)
		})
	}
}

// TestList_TenantScopeForwardedToRepository pins that a resolved tenant reaches
// the repository instead of being dropped (the pre-fix path returned every
// tenant's events).
func TestList_TenantScopeForwardedToRepository(t *testing.T) {
	repo := &auditRepo{
		items: []*domain.WebhookEvent{{ID: auditTenantEventID}},
		total: 1,
	}
	ctx := tenantctx.WithTenantScope(context.Background(), auditTenantOrgID, "scope-org", tenantctx.TenantFilterStrict)

	items, total, err := newAuditService(t, repo).List(ctx, ListQuery{
		Page: 1, PageSize: 10, WebhookID: "wh-1", OrganizationID: auditTenantOrgID,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Len(t, repo.queries, 1)
	assert.Equal(t, auditTenantOrgID, repo.queries[0].OrganizationID)
	assert.Equal(t, "wh-1", repo.queries[0].WebhookID)
}
