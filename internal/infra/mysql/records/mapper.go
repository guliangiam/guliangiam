package records

import (
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"

	domain "github.com/guliangiam/guliangiam/internal/domain/records"
)

// toRecord 把 MySQL 表扫描结果转成 domain.Record。
// metadata JSON 列从 []byte 反序列化为 map[string]any。
func toRecord(row recordRow) (*domain.Record, error) {
	actorID := ""
	if row.ActorID != nil {
		actorID = *row.ActorID
	}
	resourceID := ""
	if row.ResourceID != nil {
		resourceID = *row.ResourceID
	}
	ip := ""
	if row.IP != nil {
		ip = *row.IP
	}
	record := &domain.Record{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		ActorID:        actorID,
		Action:         row.Action,
		ResourceType:   row.ResourceType,
		ResourceID:     resourceID,
		IP:             ip,
		UserAgent:      row.UserAgent,
		Surface:        row.Surface,
		RequestID:      row.RequestID,
		TraceID:        row.TraceID,
		CreatedAt:      row.CreatedAt,
	}
	if len(row.Metadata) > 0 {
		if err := mysqlshared.UnmarshalJSON(row.Metadata, &record.Metadata); err != nil {
			return nil, err
		}
	}
	if record.Metadata == nil {
		record.Metadata = map[string]any{}
	}
	return record, nil
}
