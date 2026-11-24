package actions

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	domain "github.com/guliangiam/guliangiam/internal/domain/actions"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository is the postgres-backed implementation of domain.Repository.
// It accepts shared.DBTX so it can be used inside or outside a transaction.
type Repository struct {
	db shared.DBTX
}

// NewRepository constructs a Repository bound to the given DBTX.
func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

// Compile-time check.
var _ domain.Repository = Repository{}

// ─── Actions: List ────────────────────────────────────────────────────────

func (r Repository) List(ctx context.Context, query domain.ListActionsQuery) ([]*domain.Action, int64, error) {
	baseArgs := []any{query.OrganizationID}
	extra := ""
	if query.TriggerType != nil && *query.TriggerType != "" {
		extra, baseArgs = shared.AppendParamCondition(extra, "trigger_type", string(*query.TriggerType), baseArgs)
	}
	if query.Status != nil && *query.Status != "" {
		extra, baseArgs = shared.AppendParamCondition(extra, "status", string(*query.Status), baseArgs)
	}

	countSQL := "SELECT count(*) " + queryListActionsBase + extra
	var total int64
	if err := r.db.QueryRow(ctx, countSQL, baseArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := shared.PageOffset(query.Page, query.PageSize)
	selectSQL := queryListActionsSelect + queryListActionsBase + extra
	selectSQL += fmt.Sprintf(queryListActionsOrderPage, len(baseArgs)+1, len(baseArgs)+2)
	args := append(append([]any{}, baseArgs...), query.PageSize, offset)

	rows, err := r.db.Query(ctx, selectSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Action, 0, query.PageSize)
	for rows.Next() {
		var a domain.Action
		var triggerStr, statusStr string
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.Name, &triggerStr, &statusStr, &a.CreatedAt, &a.UpdatedAt, &a.Revision); err != nil {
			return nil, 0, err
		}
		a.TriggerType = domain.TriggerType(triggerStr)
		a.Status = domain.ActionStatus(statusStr)
		items = append(items, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ─── Actions: Get ─────────────────────────────────────────────────────────

func (r Repository) Get(ctx context.Context, id, organizationID string) (*domain.Action, error) {
	var a domain.Action
	var triggerStr, statusStr string
	if err := r.db.QueryRow(ctx, queryGetAction, id, organizationID).Scan(
		&a.ID, &a.OrganizationID, &a.Name, &triggerStr, &statusStr, &a.CreatedAt, &a.UpdatedAt, &a.Revision,
	); err != nil {
		return nil, err
	}
	a.TriggerType = domain.TriggerType(triggerStr)
	a.Status = domain.ActionStatus(statusStr)
	return &a, nil
}

// ─── Actions: Create (+ initial version) ──────────────────────────────────

func (r Repository) Create(ctx context.Context, action *domain.Action, version *domain.ActionVersion) error {
	_, err := r.db.Exec(ctx, queryInsertAction,
		action.ID, action.OrganizationID, action.Name, string(action.TriggerType), string(action.Status),
	)
	if err != nil {
		return err
	}
	if version != nil {
		_, err = r.db.Exec(ctx, queryInsertVersion,
			version.ID, version.ActionID, version.Version, version.Code,
		)
	}
	return err
}

// ─── Actions: UpdateStatus ────────────────────────────────────────────────

func (r Repository) UpdateStatus(ctx context.Context, cmd domain.UpdateStatusCommand) error {
	tag, err := r.db.Exec(ctx, queryUpdateActionStatus,
		string(cmd.Status), cmd.ActionID, cmd.OrganizationID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("action not found")
	}
	return nil
}

// ─── Versions: List ────────────────────────────────────────────────────────

func (r Repository) ListVersions(ctx context.Context, query domain.ListActionVersionsQuery) ([]*domain.ActionVersion, int64, error) {
	countSQL := "SELECT count(*) " + queryListVersionsBase
	var total int64
	if err := r.db.QueryRow(ctx, countSQL, query.ActionID, query.OrganizationID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := shared.PageOffset(query.Page, query.PageSize)
	selectSQL := queryListVersionsSelect + queryListVersionsBase + " ORDER BY version DESC"
	selectSQL += fmt.Sprintf(queryLimitOffsetPage, 3, 4)
	args := []any{query.ActionID, query.OrganizationID, query.PageSize, offset}

	rows, err := r.db.Query(ctx, selectSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.ActionVersion, 0, query.PageSize)
	for rows.Next() {
		var v domain.ActionVersion
		if err := rows.Scan(&v.ID, &v.ActionID, &v.Version, &v.Code, &v.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, &v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ─── Versions: Publish ──────────────────────────────────────────────────────

func (r Repository) PublishVersion(ctx context.Context, actionID, organizationID string, version *domain.ActionVersion) error {
	// organizationID is unused for versions (enforced via FK + RLS predicate); we keep it for call-site symmetry.
	_ = organizationID
	_, err := r.db.Exec(ctx, queryInsertVersion,
		version.ID, version.ActionID, version.Version, version.Code,
	)
	return err
}

// ─── Versions: GetLatest ──────────────────────────────────────────────────

func (r Repository) GetLatestVersion(ctx context.Context, actionID, organizationID string) (*domain.ActionVersion, error) {
	_ = organizationID
	var v domain.ActionVersion
	if err := r.db.QueryRow(ctx, queryLatestVersion, actionID).Scan(
		&v.ID, &v.ActionID, &v.Version, &v.Code, &v.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &v, nil
}

// ─── ExecutionLogs: List ───────────────────────────────────────────────────

func (r Repository) ListExecutionLogs(ctx context.Context, query domain.ListExecutionLogsQuery) ([]*domain.ExecutionLog, int64, error) {
	baseArgs := []any{query.ActionID, query.OrganizationID}
	extra := ""
	if query.Status != nil && *query.Status != "" {
		extra, baseArgs = shared.AppendParamCondition(extra, "status", string(*query.Status), baseArgs)
	}

	countSQL := "SELECT count(*) " + queryListExecLogsBase + extra
	var total int64
	if err := r.db.QueryRow(ctx, countSQL, baseArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := shared.PageOffset(query.Page, query.PageSize)
	selectSQL := queryListExecLogsSelect + queryListExecLogsBase + extra + " ORDER BY created_at DESC"
	selectSQL += fmt.Sprintf(queryLimitOffsetPage, len(baseArgs)+1, len(baseArgs)+2)
	args := append(append([]any{}, baseArgs...), query.PageSize, offset)

	rows, err := r.db.Query(ctx, selectSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.ExecutionLog, 0, query.PageSize)
	for rows.Next() {
		var l domain.ExecutionLog
		var statusStr string
		var executedAt pgtype.Timestamptz
		if err := rows.Scan(&l.ID, &l.ActionID, &l.Version, &statusStr, &l.Input, &l.Output, &l.Error, &executedAt, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		l.Status = domain.ExecutionStatus(statusStr)
		if executedAt.Valid {
			t := executedAt.Time
			l.ExecutedAt = &t
		}
		items = append(items, &l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ─── ExecutionLogs: Create ─────────────────────────────────────────────────

func (r Repository) CreateExecutionLog(ctx context.Context, log *domain.ExecutionLog) error {
	_, err := r.db.Exec(ctx, queryInsertExecLog,
		log.ID, log.ActionID, log.Version, log.Input,
	)
	return err
}

// ─── ExecutionLogs: Update ─────────────────────────────────────────────────

func (r Repository) UpdateExecutionLog(ctx context.Context, cmd domain.UpdateExecutionLogCommand) error {
	_, err := r.db.Exec(ctx, queryUpdateExecLog,
		string(cmd.Status), cmd.Output, cmd.Error, cmd.LogID,
	)
	return err
}
