package captchaproviders

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type Repository interface {
	GetByID(ctx context.Context, id, organizationID string) (*CaptchaProvider, error)
	List(ctx context.Context, query ListQuery) ([]*CaptchaProvider, int64, error)
	Create(ctx context.Context, provider *CaptchaProvider) error
	Update(ctx context.Context, provider *CaptchaProvider) error
	Delete(ctx context.Context, id, organizationID string) error
}
