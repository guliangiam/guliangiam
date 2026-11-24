package certs

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/certs"
)

type ListCertsQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Scope          string
	Type           string
}

func (s Service) ListCerts(ctx context.Context, query ListCertsQuery) ([]CertListItem, int64, error) {
	certs, total, err := s.UOW.Certs().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}

	items := make([]CertListItem, 0, len(certs))
	for _, cert := range certs {
		items = append(items, toListItem(cert))
	}

	return items, total, nil
}
