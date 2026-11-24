package risk

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	domain "github.com/guliangiam/guliangiam/internal/domain/risk"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type Repository struct {
	db mysqlshared.DBTX
}

func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

var _ domain.Repository = Repository{}

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
	_, err := r.db.ExecContext(ctx, queryAppendSignal,
		signal.ID, signal.OrganizationID, signal.UserID,
		signal.SignalType, signal.SignalValue, signal.IP, signal.CreatedAt,
	)
	return err
}

func (r Repository) CreateDecision(ctx context.Context, decision domain.Decision) error {
	_, err := r.db.ExecContext(ctx, queryCreateDecision,
		decision.ID, decision.OrganizationID, decision.UserID,
		decision.Score, string(decision.Decision), decision.Factors, decision.CreatedAt,
	)
	return err
}

func (r Repository) ListSignals(ctx context.Context, query domain.ListSignalsQuery) ([]*domain.Signal, int64, error) {
	baseArgs := []any{query.OrganizationID}
	extra := ""
	if query.UserID != nil && *query.UserID != "" {
		extra += " AND user_id = ?"
		baseArgs = append(baseArgs, *query.UserID)
	}
	if query.SignalType != nil && *query.SignalType != "" {
		extra += " AND signal_type = ?"
		baseArgs = append(baseArgs, *query.SignalType)
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT count(*) "+queryListSignalsBase+extra, baseArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	selectSQL := queryListSignalsSelect + queryListSignalsBase + extra + " ORDER BY created_at DESC, id DESC " + queryListSignalsOrderPage
	args := append(append([]any{}, baseArgs...), query.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, selectSQL, args...)
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
		extra += " AND user_id = ?"
		baseArgs = append(baseArgs, *query.UserID)
	}
	if query.Decision != nil && *query.Decision != "" {
		extra += " AND decision = ?"
		baseArgs = append(baseArgs, string(*query.Decision))
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT count(*) "+queryListDecisionsBase+extra, baseArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	selectSQL := queryListDecisionsSelect + queryListDecisionsBase + extra + " ORDER BY created_at DESC, id DESC " + queryListSignalsOrderPage
	args := append(append([]any{}, baseArgs...), query.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, selectSQL, args...)
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
	decision, err := scanDecision(r.db.QueryRowContext(ctx, statement, id, organizationID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.NewNotFound("risk decision not found")
	}
	return decision, err
}

func (r Repository) UpdateDecision(ctx context.Context, cmd domain.UpdateDecisionCommand) error {
	result, err := r.db.ExecContext(ctx, queryUpdateDecision,
		string(cmd.NewDecision), cmd.NewFactors, cmd.DecisionID, cmd.OrganizationID,
		string(cmd.ExpectedDecision), cmd.ExpectedFactors,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("risk decision revision mismatch")
	}
	return nil
}

func (r Repository) ListGovernance(ctx context.Context, query domain.ListGovernanceQuery) ([]*domain.SessionGovernance, int64, error) {
	baseArgs := []any{query.OrganizationID}
	extra := ""
	if query.Action != nil && *query.Action != "" {
		extra += " AND g.action = ?"
		baseArgs = append(baseArgs, string(*query.Action))
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT count(*) "+queryListGovernanceBase+extra, baseArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	selectSQL := queryListGovernanceSelect + queryListGovernanceBase + extra + " ORDER BY g.created_at DESC, g.id DESC " + queryListSignalsOrderPage
	args := append(append([]any{}, baseArgs...), query.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, selectSQL, args...)
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
	governance, err := scanGovernance(r.db.QueryRowContext(ctx, queryGetGovernance, id, organizationID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.NewNotFound("session governance not found")
	}
	return governance, err
}

func (r Repository) GetGovernanceForTransition(ctx context.Context, id, organizationID string) (*domain.SessionGovernance, error) {
	target, err := scanGovernance(r.db.QueryRowContext(ctx, queryLockGovernance, id, organizationID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.NewNotFound("session governance not found")
	}
	if err != nil {
		return nil, err
	}
	if target.SessionID == nil || *target.SessionID == "" {
		return nil, apperrors.NewConflict("detached session governance cannot transition")
	}
	current, err := scanGovernance(r.db.QueryRowContext(ctx, queryGetLatestGovernance, organizationID, *target.SessionID))
	if err != nil {
		return nil, err
	}
	if current.ID != target.ID {
		return nil, apperrors.NewConflict("session governance revision mismatch")
	}
	return current, nil
}

func (r Repository) AppendGovernance(ctx context.Context, governance domain.SessionGovernance) error {
	_, err := r.db.ExecContext(ctx, queryAppendGovernance,
		governance.ID, governance.OrganizationID, governance.SessionID,
		governance.RiskScore, string(governance.Action), governance.Reason, governance.CreatedAt,
	)
	return err
}

func (r Repository) ListAuditChain(ctx context.Context, query domain.AuditIntegrityQuery) (*domain.AuditChain, error) {
	rows, err := r.db.QueryContext(ctx, queryListAuditChain, query.OrganizationID, query.ChainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	chain := &domain.AuditChain{Entries: make([]domain.AuditChainRecord, 0)}
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
				return nil, fmt.Errorf("decode audit metadata at chain seq %d: %w", entry.ChainSeq, err)
			}
		}
		chain.Entries = append(chain.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return chain, nil
}
