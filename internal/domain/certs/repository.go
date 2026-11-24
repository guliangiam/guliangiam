package certs

import (
	"context"
	"time"
)

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Scope          string
	Type           string
}

type Repository interface {
	GetByID(ctx context.Context, id, organizationID string) (*Cert, error)
	List(ctx context.Context, query ListQuery) ([]*Cert, int64, error)
	Create(ctx context.Context, cert *Cert) error
	Update(ctx context.Context, cert *Cert) error
	Delete(ctx context.Context, id, organizationID string) error
}

// ExpiringScanner 是后台到期提醒的可选扫描能力，只由维护作业在系统作用域调用。
type ExpiringScanner interface {
	// ListExpiring 返回 expire_time 落在 (now, deadline] 的证书，按到期时间升序；
	// limit 限制单次扫描行数。
	ListExpiring(ctx context.Context, now, deadline time.Time, limit int) ([]*Cert, error)
}
