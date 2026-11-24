package verifications

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/verifications"
)

type ListVerificationsQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Type           string
	Status         string
	Recipient      string
}

func (s Service) ListVerifications(ctx context.Context, query ListVerificationsQuery) ([]*VerificationDTO, int64, error) {
	items, total, err := s.UOW.Verifications().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]*VerificationDTO, len(items))
	for i, v := range items {
		dtos[i] = toVerificationDTO(v)
	}
	return dtos, total, nil
}
