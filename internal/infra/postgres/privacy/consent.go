package privacy

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/privacy"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

const queryListConsentRecords = `
SELECT id, organization_id, subject_type, subject_id, purpose, version, granted_at, withdrawn_at, created_at, updated_at, revision
FROM guliangiam.consent_records
WHERE organization_id = $1::uuid
  AND subject_type = $2
  AND subject_id = $3::uuid
  AND ($4 = '' OR purpose = $4)
ORDER BY purpose ASC, version DESC
LIMIT $5`

const queryLatestConsentRecord = `
SELECT id, organization_id, subject_type, subject_id, purpose, version, granted_at, withdrawn_at, created_at, updated_at, revision
FROM guliangiam.consent_records
WHERE organization_id = $1::uuid
  AND subject_type = $2
  AND subject_id = $3::uuid
  AND purpose = $4
ORDER BY version DESC
LIMIT 1`

const queryInsertConsentRecord = `
INSERT INTO guliangiam.consent_records
    (id, organization_id, subject_type, subject_id, purpose, version, granted_at, withdrawn_at, created_at, updated_at, revision)
VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5, $6, $7, $8, $9, $10, $11)`

const queryWithdrawLatestConsent = `
UPDATE guliangiam.consent_records
SET withdrawn_at = $5, updated_at = $5, revision = revision + 1
WHERE id = (
    SELECT id FROM guliangiam.consent_records
    WHERE organization_id = $1::uuid AND subject_type = $2 AND subject_id = $3::uuid AND purpose = $4
    ORDER BY version DESC LIMIT 1
)
AND withdrawn_at IS NULL
RETURNING id, organization_id, subject_type, subject_id, purpose, version, granted_at, withdrawn_at, created_at, updated_at, revision`

// queryConsentSubjectLock 串行化同一 (org, subject, purpose) 的并发 Grant：
// read-then-write 版本号在无锁时可重号。事务级 advisory lock，提交/回滚自动释放。
const queryConsentSubjectLock = `SELECT pg_advisory_xact_lock(hashtext($1))`

// ConsentRepository persists consent_records on the DBTX it is bound to.
type ConsentRepository struct {
	db shared.DBTX
}

func NewConsentRepository(db shared.DBTX) ConsentRepository {
	return ConsentRepository{db: db}
}

func scanConsentRecord(row scanner) (*domain.ConsentRecord, error) {
	var record domain.ConsentRecord
	if err := row.Scan(&record.ID, &record.OrganizationID, &record.SubjectType, &record.SubjectID,
		&record.Purpose, &record.Version, &record.GrantedAt, &record.WithdrawnAt,
		&record.CreatedAt, &record.UpdatedAt, &record.Revision); err != nil {
		return nil, err
	}
	return &record, nil
}

// List returns the version chain for one subject (newest version first per purpose).
func (r ConsentRepository) List(ctx context.Context, organizationID string, query domain.ConsentQuery) ([]*domain.ConsentRecord, error) {
	limit := query.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, queryListConsentRecords,
		organizationID, query.SubjectType, query.SubjectID, query.Purpose, limit)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list consent records")
	}
	defer rows.Close()
	records := make([]*domain.ConsentRecord, 0, limit)
	for rows.Next() {
		record, err := scanConsentRecord(rows)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "scan consent record")
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "iterate consent records")
	}
	return records, nil
}

func (r ConsentRepository) LockSubject(ctx context.Context, organizationID, subjectType, subjectID, purpose string) error {
	key := "consent:" + organizationID + ":" + subjectType + ":" + subjectID + ":" + purpose
	if _, err := r.db.Exec(ctx, queryConsentSubjectLock, key); err != nil {
		return apperrors.WrapInternal(err, "acquire consent subject advisory lock")
	}
	return nil
}

func (r ConsentRepository) Latest(ctx context.Context, organizationID, subjectType, subjectID, purpose string) (*domain.ConsentRecord, error) {
	record, err := scanConsentRecord(r.db.QueryRow(ctx, queryLatestConsentRecord, organizationID, subjectType, subjectID, purpose))
	if err == pgx.ErrNoRows {
		return nil, apperrors.NewNotFound("consent record not found")
	}
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "get latest consent record")
	}
	return record, nil
}

func (r ConsentRepository) Create(ctx context.Context, record *domain.ConsentRecord) error {
	if _, err := r.db.Exec(ctx, queryInsertConsentRecord,
		record.ID, record.OrganizationID, record.SubjectType, record.SubjectID, record.Purpose,
		record.Version, record.GrantedAt, record.WithdrawnAt, record.CreatedAt, record.UpdatedAt, record.Revision,
	); err != nil {
		return apperrors.WrapInternalf(err, "insert consent record")
	}
	return nil
}

func (r ConsentRepository) WithdrawLatest(ctx context.Context, organizationID, subjectType, subjectID, purpose string, at time.Time) (*domain.ConsentRecord, error) {
	record, err := scanConsentRecord(r.db.QueryRow(ctx, queryWithdrawLatestConsent,
		organizationID, subjectType, subjectID, purpose, at))
	if err == pgx.ErrNoRows {
		return nil, apperrors.NewNotFound("no active consent to withdraw")
	}
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "withdraw consent record")
	}
	return record, nil
}

var _ domain.ConsentRepository = ConsentRepository{}
