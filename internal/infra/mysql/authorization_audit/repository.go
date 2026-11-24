package auditrepo

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/guliangiam/guliangiam/internal/domain/authorization_audit"
)

// Repository is the MySQL mirror of the authorization audit hash chain.
type Repository struct {
	db *sql.DB
}

var _ domain.AuditRepository = Repository{}

func NewRepository(db *sql.DB) Repository {
	return Repository{db: db}
}

// Append writes one audit record and advances the chain head atomically.
func (r Repository) Append(ctx context.Context, rec *domain.AuditRecord, createdAt string) error {
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		created, err = time.Parse(time.RFC3339, createdAt)
	}
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, queryChainStateInit); err != nil {
		return err
	}
	var prevHash *string
	if err := tx.QueryRowContext(ctx, queryChainStateLock).Scan(&prevHash); err != nil {
		return err
	}
	hash := computeChainHash(prevHash, createdAt, rec)
	var sourceTenant, targetTenant any
	if rec.SourceTenant != nil && *rec.SourceTenant != uuid.Nil {
		sourceTenant = rec.SourceTenant.String()
	}
	if rec.TargetTenant != nil && *rec.TargetTenant != uuid.Nil {
		targetTenant = rec.TargetTenant.String()
	}
	if _, err := tx.ExecContext(ctx, queryInsertAuditRecord,
		uuid.NewString(), rec.ActorID.String(), rec.ActorType, sourceTenant, targetTenant,
		rec.ResourceType, rec.Action, rec.RiskLevel,
		rec.Decision, rec.Reason, prevHash, hash, created,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, queryUpdateChainState, hash); err != nil {
		return err
	}
	return tx.Commit()
}

// ListRecords returns the newest audit-chain records.
func (r Repository) ListRecords(ctx context.Context, limit int) ([]domain.ChainRecord, error) {
	rows, err := r.db.QueryContext(ctx, queryListForValidate, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []domain.ChainRecord
	for rows.Next() {
		var record domain.ChainRecord
		if err := rows.Scan(
			&record.ActorID, &record.ActorType, &record.SourceTenant, &record.TargetTenant,
			&record.ResourceType, &record.Action, &record.RiskLevel, &record.Decision, &record.Reason,
			&record.PrevHash, &record.Hash, &record.CreatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func computeChainHash(prevHash *string, createdAt string, rec *domain.AuditRecord) string {
	parts := []string{
		derefOrEmpty(prevHash), createdAt, uuidOrEmpty(rec.ActorID), rec.ActorType,
		uuidPtrOrEmpty(rec.SourceTenant), uuidPtrOrEmpty(rec.TargetTenant),
		rec.ResourceType, rec.Action, rec.RiskLevel, rec.Decision, derefOrEmpty(rec.Reason),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func uuidOrEmpty(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

func uuidPtrOrEmpty(id *uuid.UUID) string {
	if id == nil || *id == uuid.Nil {
		return ""
	}
	return id.String()
}

const (
	queryChainStateInit    = `INSERT IGNORE INTO authorization_chain_state (id, last_hash) VALUES (1, NULL)`
	queryChainStateLock    = `SELECT last_hash FROM authorization_chain_state WHERE id = 1 FOR UPDATE`
	queryInsertAuditRecord = `
INSERT INTO authorization_audit_records (
  id, actor_id, actor_type, source_tenant, target_tenant,
  resource_type, action, risk_level, scope_id, ticket_id,
  decision, reason, prev_hash, hash, trace_id, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, ?, ?, ?, ?, NULL, ?)`
	queryUpdateChainState = `UPDATE authorization_chain_state SET last_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1`
	queryListForValidate  = `
SELECT actor_id, actor_type, source_tenant, target_tenant,
       resource_type, action, risk_level, decision, reason,
       prev_hash, hash, created_at
FROM authorization_audit_records
ORDER BY created_at DESC
LIMIT ?`
)
