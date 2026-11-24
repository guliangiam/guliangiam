package records

import (
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/records"
)

// RecordDTO is the data transfer object returned by the service layer,
// decoupling consumers from the domain entity.
type RecordDTO struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organizationId"`
	ActorID        string         `json:"actorId"`
	Action         string         `json:"action"`
	ResourceType   string         `json:"resourceType"`
	ResourceID     string         `json:"resourceId"`
	IP             string         `json:"ip"`
	UserAgent      string         `json:"userAgent"`
	Surface        string         `json:"surface"`
	RequestID      string         `json:"requestId"`
	TraceID        string         `json:"traceId"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"createdAt"`
}

func toRecordDTO(r *domain.Record) *RecordDTO {
	if r == nil {
		return nil
	}
	return &RecordDTO{
		ID:             r.ID,
		OrganizationID: r.OrganizationID,
		ActorID:        r.ActorID,
		Action:         r.Action,
		ResourceType:   r.ResourceType,
		ResourceID:     r.ResourceID,
		IP:             r.IP,
		UserAgent:      r.UserAgent,
		Surface:        r.Surface,
		RequestID:      r.RequestID,
		TraceID:        r.TraceID,
		Metadata:       r.Metadata,
		CreatedAt:      r.CreatedAt,
	}
}
