package crosstenantops

import (
	"context"
	"errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
	pgtenant "github.com/guliangiam/guliangiam/internal/infra/postgres"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// Repository — TicketRepository 的 Postgres 实现。
// 所有方法开头自动注入 TenantFilterSystem ctx（RLS: is_super_admin() 才能读写）。
type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

// sysCtx 注入 TenantFilterSystem — RLS: is_super_admin() 才能读写。
func sysCtx(ctx context.Context) context.Context {
	return pgtenant.WithTenantScope(ctx, "", "", pgtenant.TenantFilterSystem)
}

// 确保实现 domain 接口。
var _ domain.TicketRepository = Repository{}

// ─── helpers ───────────────────────────────────────────────────────────────

// scanTicketRows 扫描 Ticket 行（SELECT 用）。
func scanTicketRows(rows pgx.Rows) ([]*domain.Ticket, error) {
	items := make([]*domain.Ticket, 0)
	defer rows.Close()
	for rows.Next() {
		t, err := scanTicket(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// scanTicket 单行扫描（用 Scan 方法闭包）。
func scanTicket(scan func(...any) error) (*domain.Ticket, error) {
	var t domain.Ticket
	err := scan(
		&t.ID, &t.ScopeID, &t.ActorID, &t.SourceTenant, &t.TargetTenant,
		&t.ResourceType, &t.Action, &t.RiskLevel, &t.Status,
		&t.ApprovedBy, &t.ApprovedAt, &t.ExpiresAt, &t.UsedAt,
		&t.TraceID, &t.Reason, &t.CreatedAt,
	)
	return &t, err
}

// ─── Hot path ──────────────────────────────────────────────────────────────

func (r Repository) HasApprovedTicket(ctx context.Context, actorUserID, targetTenant, resourceType, action string) (bool, error) {
	ctx = sysCtx(ctx)
	var has bool

	var tenantArg any = nil
	if targetTenant != "" {
		tenantArg = targetTenant
	}

	err := r.db.QueryRow(ctx, queryHasApprovedTicket, actorUserID, tenantArg, resourceType, action).Scan(&has)
	return has, err
}

// ─── CRUD ─────────────────────────────────────────────────────────────────

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Ticket, error) {
	ctx = sysCtx(ctx)
	var t domain.Ticket
	err := r.db.QueryRow(ctx, queryGetByID, id).Scan(
		&t.ID, &t.ScopeID, &t.ActorID, &t.SourceTenant, &t.TargetTenant,
		&t.ResourceType, &t.Action, &t.RiskLevel, &t.Status,
		&t.ApprovedBy, &t.ApprovedAt, &t.ExpiresAt, &t.UsedAt,
		&t.TraceID, &t.Reason, &t.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NewNotFound("cross-tenant operation not found")
	}
	return &t, err
}

func (r Repository) Create(ctx context.Context, ticket *domain.Ticket) error {
	ctx = sysCtx(ctx)
	_, err := r.db.Exec(ctx, queryInsert,
		ticket.ID, ticket.ScopeID, ticket.ActorID, ticket.SourceTenant, ticket.TargetTenant,
		ticket.ResourceType, ticket.Action, ticket.RiskLevel, ticket.Status,
		ticket.ApprovedBy, ticket.ApprovedAt, ticket.ExpiresAt, ticket.UsedAt,
		ticket.TraceID, ticket.Reason, ticket.CreatedAt,
	)
	return err
}

func (r Repository) Approve(ctx context.Context, ticketID, approverPlatformAdminID string) error {
	ctx = sysCtx(ctx)
	result, err := r.db.Exec(ctx, queryApprove, approverPlatformAdminID, ticketID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewForbidden("cannot approve: ticket not found or not pending")
	}
	return nil
}

func (r Repository) Reject(ctx context.Context, ticketID, reason string) error {
	ctx = sysCtx(ctx)
	result, err := r.db.Exec(ctx, queryReject, reason, ticketID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewForbidden("cannot reject: ticket not found or not pending")
	}
	return nil
}

func (r Repository) MarkUsed(ctx context.Context, ticketID string) error {
	ctx = sysCtx(ctx)
	result, err := r.db.Exec(ctx, queryMarkUsed, ticketID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewForbidden("cannot mark used: ticket not found or not approved")
	}
	return nil
}

func (r Repository) ListByActor(ctx context.Context, actorUserID, status string) ([]*domain.Ticket, error) {
	ctx = sysCtx(ctx)
	rows, err := r.db.Query(ctx, queryListByActor, actorUserID, status)
	if err != nil {
		return nil, err
	}
	return scanTicketRows(rows)
}

func (r Repository) ListByScope(ctx context.Context, scopeID string) ([]*domain.Ticket, error) {
	ctx = sysCtx(ctx)
	rows, err := r.db.Query(ctx, queryListByScope, scopeID)
	if err != nil {
		return nil, err
	}
	return scanTicketRows(rows)
}
