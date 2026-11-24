package records

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/records"
)

func toRecord(row recordRow) domain.Record {
	ip := ""
	if row.IP != nil {
		ip = *row.IP
	}
	actorID := ""
	if row.ActorID != nil {
		actorID = *row.ActorID
	}
	resourceID := ""
	if row.ResourceID != nil {
		resourceID = *row.ResourceID
	}
	return domain.Record{
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
		Metadata:       row.Metadata,
		CreatedAt:      row.CreatedAt,
	}
}
