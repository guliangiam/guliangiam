package privacy

import (
	"context"
	"time"
)

// DsarRepository persists the DSAR aggregate together with its immutable event.
// Every write must commit the request row and its event row atomically; the
// audit record is embedded by the application layer in the same transaction so
// the compliance trail cannot diverge from the state change (design/privacy §7).
type DsarRepository interface {
	Create(ctx context.Context, request *DsarRequest, event *DsarEvent) error
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*DsarRequest, error)
	UpdateWithEvent(ctx context.Context, request *DsarRequest, event *DsarEvent, expectedRevision int64) error
	ListOverdue(ctx context.Context, now time.Time, limit int) ([]*DsarRequest, error)
	ListByOrganization(ctx context.Context, organizationID string, limit, offset int) ([]*DsarRequest, int64, error)
}

// SubjectDataCollector is implemented by the privacy repository when it can
// aggregate one data subject's records across tenant tables for an evidence
// export. It deliberately returns no credentials/secrets (design/privacy §4).
type SubjectDataCollector interface {
	CollectSubjectData(ctx context.Context, organizationID, subjectType, subjectID string) ([]ExportRecord, string, error)
}
