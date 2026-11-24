package records

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/records"
)

type SearchRecordsCriteria struct {
	Page           int
	PageSize       int
	OrganizationID string
	ActorID        string
	Action         string
	ResourceType   string
	ResourceID     string
	RequestID      string
	Keyword        string
	SortField      string
	SortOrder      string
	StartTime      *time.Time
	EndTime        *time.Time
}

func (s Service) SearchRecords(ctx context.Context, criteria SearchRecordsCriteria) ([]*RecordDTO, int64, error) {
	items, total, err := s.UOW.Records().Search(ctx, domain.SearchCriteria(criteria))
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]*RecordDTO, len(items))
	for i, r := range items {
		dtos[i] = toRecordDTO(r)
	}
	return dtos, total, nil
}
