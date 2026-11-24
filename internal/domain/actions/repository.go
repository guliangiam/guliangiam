package actions

import "context"

// Repository is the persistence contract for identity actions, their
// versions, and execution logs. All methods enforce multi-tenancy via
// explicit organization_id — callers must supply it.
type Repository interface {
	// Actions
	List(ctx context.Context, query ListActionsQuery) ([]*Action, int64, error)
	Get(ctx context.Context, id, organizationID string) (*Action, error)
	Create(ctx context.Context, action *Action, version *ActionVersion) error
	UpdateStatus(ctx context.Context, cmd UpdateStatusCommand) error

	// Versions
	ListVersions(ctx context.Context, query ListActionVersionsQuery) ([]*ActionVersion, int64, error)
	PublishVersion(ctx context.Context, actionID, organizationID string, version *ActionVersion) error
	GetLatestVersion(ctx context.Context, actionID, organizationID string) (*ActionVersion, error)

	// Execution logs
	ListExecutionLogs(ctx context.Context, query ListExecutionLogsQuery) ([]*ExecutionLog, int64, error)
	CreateExecutionLog(ctx context.Context, log *ExecutionLog) error
	UpdateExecutionLog(ctx context.Context, cmd UpdateExecutionLogCommand) error
}
