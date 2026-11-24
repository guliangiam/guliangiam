package risk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/risk"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

var _ domain.Repository = Repository{}
var _ domain.AuditChainSegmentReader = Repository{}

type scanner interface {
	Scan(dest ...any) error
}

func scanDecision(row scanner) (*domain.Decision, error) {
	var d domain.Decision
	var decision string
	if err := row.Scan(&d.ID, &d.OrganizationID, &d.UserID, &d.Score, &decision, &d.Factors, &d.CreatedAt); err != nil {
		return nil, err
	}
	d.Decision = domain.DecisionAction(decision)
	return &d, nil
}

func scanGovernance(row scanner) (*domain.SessionGovernance, error) {
	var g domain.SessionGovernance
	var action string
	if err := row.Scan(&g.ID, &g.OrganizationID, &g.SessionID, &g.RiskScore, &action, &g.Reason, &g.CreatedAt, &g.Revision); err != nil {
		return nil, err
	}
	g.Action = domain.GovernanceAction(action)
	return &g, nil
}

func (r Repository) AppendSignal(ctx context.Context, signal domain.Signal) error {
	_, err := r.db.Exec(ctx, queryAppendSignal,
		signal.ID, signal.OrganizationID, signal.UserID,
		signal.SignalType, signal.SignalValue, signal.IP, signal.CreatedAt,
	)
	return err
}

func (r Repository) CreateDecision(ctx context.Context, decision domain.Decision) error {
	_, err := r.db.Exec(ctx, queryCreateDecision,
		decision.ID, decision.OrganizationID, decision.UserID,
		decision.Score, string(decision.Decision), decision.Factors, decision.CreatedAt,
	)
	return err
}

func (r Repository) ListSignals(ctx context.Context, query domain.ListSignalsQuery) ([]*domain.Signal, int64, error) {
	baseArgs := []any{query.OrganizationID}
	extra := ""
	if query.UserID != nil && *query.UserID != "" {
		extra += " AND user_id = $2"
		baseArgs = append(baseArgs, *query.UserID)
	}
	if query.SignalType != nil && *query.SignalType != "" {
		extra, baseArgs = shared.AppendParamCondition(extra, "signal_type", *query.SignalType, baseArgs)
	}

	var total int64
	if err := r.db.QueryRow(ctx, "SELECT count(*) "+queryListSignalsBase+extra, baseArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := shared.PageOffset(query.Page, query.PageSize)
	selectSQL := queryListSignalsSelect + queryListSignalsBase + extra + " ORDER BY created_at DESC, id DESC"
	selectSQL += fmt.Sprintf(queryListSignalsOrderPage, len(baseArgs)+1, len(baseArgs)+2)
	args := append(append([]any{}, baseArgs...), query.PageSize, offset)
	rows, err := r.db.Query(ctx, selectSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Signal, 0, query.PageSize)
	for rows.Next() {
		var signal domain.Signal
		if err := rows.Scan(&signal.ID, &signal.OrganizationID, &signal.UserID, &signal.SignalType, &signal.SignalValue, &signal.IP, &signal.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, &signal)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) ListDecisions(ctx context.Context, query domain.ListDecisionsQuery) ([]*domain.Decision, int64, error) {
	baseArgs := []any{query.OrganizationID}
	extra := ""
	if query.UserID != nil && *query.UserID != "" {
		extra += " AND user_id = $2"
		baseArgs = append(baseArgs, *query.UserID)
	}
	if query.Decision != nil && *query.Decision != "" {
		extra, baseArgs = shared.AppendParamCondition(extra, "decision", string(*query.Decision), baseArgs)
	}

	var total int64
	if err := r.db.QueryRow(ctx, "SELECT count(*) "+queryListDecisionsBase+extra, baseArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := shared.PageOffset(query.Page, query.PageSize)
	selectSQL := queryListDecisionsSelect + queryListDecisionsBase + extra + " ORDER BY created_at DESC, id DESC"
	selectSQL += fmt.Sprintf(queryListSignalsOrderPage, len(baseArgs)+1, len(baseArgs)+2)
	args := append(append([]any{}, baseArgs...), query.PageSize, offset)
	rows, err := r.db.Query(ctx, selectSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Decision, 0, query.PageSize)
	for rows.Next() {
		decision, err := scanDecision(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, decision)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) GetDecision(ctx context.Context, id, organizationID string) (*domain.Decision, error) {
	return r.getDecision(ctx, queryGetDecision, id, organizationID)
}

func (r Repository) GetDecisionForUpdate(ctx context.Context, id, organizationID string) (*domain.Decision, error) {
	return r.getDecision(ctx, queryGetDecisionForUpdate, id, organizationID)
}

func (r Repository) getDecision(ctx context.Context, statement, id, organizationID string) (*domain.Decision, error) {
	decision, err := scanDecision(r.db.QueryRow(ctx, statement, id, organizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NewNotFound("risk decision not found")
	}
	return decision, err
}

func (r Repository) UpdateDecision(ctx context.Context, cmd domain.UpdateDecisionCommand) error {
	result, err := r.db.Exec(ctx, queryUpdateDecision,
		string(cmd.NewDecision), cmd.NewFactors, cmd.DecisionID, cmd.OrganizationID,
		string(cmd.ExpectedDecision), cmd.ExpectedFactors,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewConflict("risk decision revision mismatch")
	}
	return nil
}

func (r Repository) ListGovernance(ctx context.Context, query domain.ListGovernanceQuery) ([]*domain.SessionGovernance, int64, error) {
	baseArgs := []any{query.OrganizationID}
	extra := ""
	if query.Action != nil && *query.Action != "" {
		extra, baseArgs = shared.AppendParamCondition(extra, "g.action", string(*query.Action), baseArgs)
	}

	var total int64
	if err := r.db.QueryRow(ctx, "SELECT count(*) "+queryListGovernanceBase+extra, baseArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := shared.PageOffset(query.Page, query.PageSize)
	// The window revision is computed inside the derived table over the full
	// tenant-scoped session history; the optional action filter is applied
	// outside so filtered-out sibling rows still count toward the revision
	// (same semantics as the correlated subquery used by single-row reads).
	filter := ""
	if extra != "" {
		filter = " WHERE " + strings.TrimPrefix(extra, " AND ")
	}
	selectSQL := queryListGovernanceWindowPage + filter + " ORDER BY g.created_at DESC, g.id DESC"
	selectSQL += fmt.Sprintf(queryListSignalsOrderPage, len(baseArgs)+1, len(baseArgs)+2)
	args := append(append([]any{}, baseArgs...), query.PageSize, offset)
	rows, err := r.db.Query(ctx, selectSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.SessionGovernance, 0, query.PageSize)
	for rows.Next() {
		governance, err := scanGovernance(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, governance)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) GetGovernance(ctx context.Context, id, organizationID string) (*domain.SessionGovernance, error) {
	governance, err := scanGovernance(r.db.QueryRow(ctx, queryGetGovernance, id, organizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NewNotFound("session governance not found")
	}
	return governance, err
}

// GetGovernanceForTransition locks the requested current row before reading the
// latest session event. A concurrent transition therefore either serializes
// behind this transaction or is rejected as stale when it resumes.
func (r Repository) GetGovernanceForTransition(ctx context.Context, id, organizationID string) (*domain.SessionGovernance, error) {
	target, err := scanGovernance(r.db.QueryRow(ctx, queryLockGovernance, id, organizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NewNotFound("session governance not found")
	}
	if err != nil {
		return nil, err
	}
	if target.SessionID == nil || *target.SessionID == "" {
		return nil, apperrors.NewConflict("detached session governance cannot transition")
	}
	current, err := scanGovernance(r.db.QueryRow(ctx, queryGetLatestGovernance, organizationID, *target.SessionID))
	if err != nil {
		return nil, err
	}
	if current.ID != target.ID {
		return nil, apperrors.NewConflict("session governance revision mismatch")
	}
	return current, nil
}

func (r Repository) AppendGovernance(ctx context.Context, governance domain.SessionGovernance) error {
	_, err := r.db.Exec(ctx, queryAppendGovernance,
		governance.ID, governance.OrganizationID, governance.SessionID,
		governance.RiskScore, string(governance.Action), governance.Reason, governance.CreatedAt,
	)
	return err
}

func (r Repository) ListAuditChain(ctx context.Context, query domain.AuditIntegrityQuery) (*domain.AuditChain, error) {
	entries, err := r.listAuditChainEntries(ctx, queryListAuditChain, query.OrganizationID, query.ChainID)
	if err != nil {
		return nil, err
	}
	anchors, err := r.ListAuditChainAnchors(ctx, query.OrganizationID, query.ChainID)
	if err != nil {
		return nil, err
	}
	return &domain.AuditChain{Entries: entries, Anchors: anchors}, nil
}

// ListAuditChainSegment loads one bounded page of the chain (BE-D-15). It is
// the read seam the app verifier uses to walk long chains without holding
// every record in memory; pending (NULL entry_hash) rows are included.
func (r Repository) ListAuditChainSegment(ctx context.Context, organizationID, chainID string, afterSeq int64, limit int) ([]domain.AuditChainRecord, error) {
	if limit <= 0 {
		return nil, apperrors.NewInvalidArgumentf("audit chain segment limit must be positive")
	}
	return r.listAuditChainEntries(ctx, queryListAuditChainSegment, organizationID, chainID, afterSeq, limit)
}

// ListAuditChainAnchors loads the (few) chain anchors in ascending order.
func (r Repository) ListAuditChainAnchors(ctx context.Context, organizationID, chainID string) ([]domain.AuditChainAnchor, error) {
	anchorRows, err := r.db.Query(ctx, queryListAuditChainAnchors, organizationID, chainID)
	if err != nil {
		return nil, err
	}
	defer anchorRows.Close()
	var anchors []domain.AuditChainAnchor
	for anchorRows.Next() {
		var anchor domain.AuditChainAnchor
		if err := anchorRows.Scan(&anchor.AnchorSeq, &anchor.EntryHash); err != nil {
			return nil, err
		}
		anchors = append(anchors, anchor)
	}
	if err := anchorRows.Err(); err != nil {
		return nil, err
	}
	return anchors, nil
}

func (r Repository) listAuditChainEntries(ctx context.Context, statement string, args ...any) ([]domain.AuditChainRecord, error) {
	rows, err := r.db.Query(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]domain.AuditChainRecord, 0)
	for rows.Next() {
		var entry domain.AuditChainRecord
		var metadata []byte
		if err := rows.Scan(
			&entry.OrganizationID, &entry.ActorID, &entry.Action, &entry.ResourceType, &entry.ResourceID,
			&entry.IP, &entry.UserAgent, &entry.Surface, &entry.RequestID, &entry.TraceID,
			&metadata, &entry.CreatedAt, &entry.ChainSeq, &entry.PrevHash, &entry.EntryHash, &entry.ChainAlgo,
		); err != nil {
			return nil, err
		}
		entry.Metadata = map[string]any{}
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &entry.Metadata); err != nil {
				return nil, apperrors.WrapInternalf(err, "decode audit metadata at chain seq %d", entry.ChainSeq)
			}
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}
