package records

import "time"

// API surfaces (手册 4.1.3). The value is stored verbatim in records.surface,
// which carries a CHECK constraint limited to these three values, so an unknown
// surface must never reach the repository.
const (
	SurfaceManagement     = "management"
	SurfaceAuthentication = "authentication"
	SurfaceProtocol       = "protocol"
)

// NormalizeSurface maps anything other than a known surface to management. The
// column default only serves historical rows (手册 12.12): every new write must
// carry an explicit value, and this fallback keeps a caller that forgot to pass
// one from failing the CHECK constraint at insert time.
func NormalizeSurface(surface string) string {
	switch surface {
	case SurfaceManagement, SurfaceAuthentication, SurfaceProtocol:
		return surface
	default:
		return SurfaceManagement
	}
}

type Record struct {
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
