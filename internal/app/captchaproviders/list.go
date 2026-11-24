package captchaproviders

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
)

type ListCaptchaProvidersQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

func (s Service) ListCaptchaProviders(ctx context.Context, query ListCaptchaProvidersQuery) ([]CaptchaProviderListItem, int64, error) {
	providers, total, err := s.UOW.CaptchaProviders().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}

	items := make([]CaptchaProviderListItem, 0, len(providers))
	for _, provider := range providers {
		items = append(items, toListItem(provider))
	}

	return items, total, nil
}
