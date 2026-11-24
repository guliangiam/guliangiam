package records

import "time"

type recordRow struct {
	ID             string
	OrganizationID string
	ActorID        *string
	Action         string
	ResourceType   string
	ResourceID     *string
	IP             *string
	UserAgent      string
	Surface        string
	RequestID      string
	TraceID        string
	Metadata       map[string]any
	CreatedAt      time.Time
}
