package invitations

import (
	"context"
	"time"
)

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Status         string
}

type Repository interface {
	// GetByID is the system-scoped lookup used by the code-based accept flow,
	// which runs outside the management by-ID tenant binding. Management
	// routes (GET/send/revoke/resend) must use GetByIDForOrganization.
	GetByID(ctx context.Context, id string) (*Invitation, error)
	// GetByIDForOrganization returns the invitation only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Invitation, error)
	GetByCode(ctx context.Context, code string) (*Invitation, error)
	GetByEmail(ctx context.Context, organizationID string, email string) (*Invitation, error)
	List(ctx context.Context, query ListQuery) ([]*Invitation, int64, error)
	Create(ctx context.Context, invitation *Invitation) error
	Update(ctx context.Context, invitation *Invitation) error
	Delete(ctx context.Context, id string) error
}

// ExpiringScanner 是后台到期提醒的可选扫描能力。它只由维护作业在系统作用域调用，
// 不并入 Repository，避免把跨租户扫描强加给每个实现。
type ExpiringScanner interface {
	// ListExpiring 返回 status=pending 且 expires_at 落在 (now, deadline] 的邀请，
	// 按到期时间升序；limit 限制单次扫描行数。
	ListExpiring(ctx context.Context, now, deadline time.Time, limit int) ([]*Invitation, error)
}
