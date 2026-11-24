package records

import (
	"time"

	apprecords "github.com/guliangiam/guliangiam/internal/app/records"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type RecordResponse struct {
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

func toResponse(record *apprecords.RecordDTO) *RecordResponse {
	return &RecordResponse{
		ID:             record.ID,
		OrganizationID: record.OrganizationID,
		ActorID:        record.ActorID,
		Action:         record.Action,
		ResourceType:   record.ResourceType,
		ResourceID:     record.ResourceID,
		IP:             record.IP,
		UserAgent:      record.UserAgent,
		Surface:        record.Surface,
		RequestID:      record.RequestID,
		TraceID:        record.TraceID,
		Metadata:       record.Metadata,
		CreatedAt:      record.CreatedAt,
	}
}

type ListRecordsResponse = presenter.ListResponse[*apprecords.RecordDTO]

type ListLoginLogsResponse = presenter.ListResponse[*apprecords.LoginLogDTO]

type ExportManifest struct {
	Count       int64          `json:"count"`
	SHA256      string         `json:"sha256"`
	GeneratedAt time.Time      `json:"generatedAt"`
	Filters     map[string]any `json:"filters"`
}

type ExportRecordsResponse struct {
	Manifest ExportManifest          `json:"manifest"`
	Records  []*apprecords.RecordDTO `json:"records"`
}
