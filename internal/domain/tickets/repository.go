package tickets

import (
	"context"
)

// ListQuery defines the query parameters for listing tickets.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	Status                  *string
	Assignee                string
	Creator                 string
	Priority                string
	Type                    string
}

// Repository defines the interface for tickets persistence operations.
type Repository interface {
	// GetByID 按 id + organizationID 读取；跨租户与不存在同样返回 NotFound。
	GetByID(ctx context.Context, id, organizationID string) (*Ticket, error)
	List(ctx context.Context, q ListQuery) ([]*Ticket, int64, error)
	Create(ctx context.Context, ticket *Ticket) error
	// Update 以实体 OrganizationID 作为租户谓词。
	Update(ctx context.Context, ticket *Ticket) error
	// Delete 按 id + organizationID 删除；0 行返回 NotFound。
	Delete(ctx context.Context, id, organizationID string) error
}
