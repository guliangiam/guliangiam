package records

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/records"
)

type ListRecordsQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	ActorID        string
	Action         string
	ResourceType   string
	RequestID      string
	Keyword        string
	SortField      string
	SortOrder      string
}

func (s Service) ListRecords(ctx context.Context, query ListRecordsQuery) ([]*RecordDTO, int64, error) {
	items, total, err := s.UOW.Records().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]*RecordDTO, len(items))
	for i, r := range items {
		dtos[i] = toRecordDTO(r)
	}
	return dtos, total, nil
}
