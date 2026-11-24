package workload

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/workload"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.4+ 实现的 workload 存储层。
type Repository struct{ db mysqlshared.DBTX }

func NewRepository(db mysqlshared.DBTX) Repository { return Repository{db: db} }

var _ domain.Repository = Repository{}

type scannable interface {
	Scan(dest ...any) error
}

func (r Repository) ListPools(ctx context.Context, q domain.ListPoolsQuery) ([]*domain.Pool, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, qCountPools, q.OrganizationID).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.QueryContext(ctx, qListPools, q.OrganizationID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []*domain.Pool{}
	for rows.Next() {
		var p domain.Pool
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.DisplayName, &p.Description, &p.CreatedAt, &p.UpdatedAt, &p.Revision); err != nil {
			return nil, 0, err
		}
		items = append(items, &p)
	}
	return items, total, rows.Err()
}

func (r Repository) GetPool(ctx context.Context, id, orgID string) (*domain.Pool, error) {
	var p domain.Pool
	if err := r.db.QueryRowContext(ctx, qGetPool, id, orgID).Scan(&p.ID, &p.OrganizationID, &p.Name, &p.DisplayName, &p.Description, &p.CreatedAt, &p.UpdatedAt, &p.Revision); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r Repository) CreatePool(ctx context.Context, p *domain.Pool) error {
	_, err := r.db.ExecContext(ctx, qInsertPool, p.ID, p.OrganizationID, p.Name, p.DisplayName, p.Description)
	return err
}

func (r Repository) ListTrusts(ctx context.Context, q domain.ListTrustsQuery) ([]*domain.FederationTrust, int64, error) {
	args := []any{q.OrganizationID, q.OrganizationID, q.PoolID, q.PoolID}
	var total int64
	if err := r.db.QueryRowContext(ctx, qCountTrusts, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, qListTrusts, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []*domain.FederationTrust{}
	for rows.Next() {
		t, err := scanTrustRow(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, t)
	}
	return items, total, rows.Err()
}

func (r Repository) GetTrust(ctx context.Context, id, orgID string) (*domain.FederationTrust, error) {
	return scanTrustRow(r.db.QueryRowContext(ctx, qGetTrust, id, orgID))
}

func (r Repository) FindActiveTrustsByIssuer(ctx context.Context, issuer string) ([]*domain.FederationTrust, error) {
	rows, err := r.db.QueryContext(ctx, qFindActiveTrustsByIssuer, issuer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*domain.FederationTrust{}
	for rows.Next() {
		t, err := scanTrustRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (r Repository) CreateTrust(ctx context.Context, t *domain.FederationTrust) error {
	audience, err := marshalStringArray(t.Audience)
	if err != nil {
		return err
	}
	subjects, err := marshalStringArray(t.Subjects)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, qInsertTrust, t.ID, t.OrganizationID, t.PoolID, t.Name, t.Issuer, t.JWKSURI, audience, subjects)
	return err
}

func scanTrustRow(s scannable) (*domain.FederationTrust, error) {
	var t domain.FederationTrust
	var audience, subjects, status string
	if err := s.Scan(&t.ID, &t.OrganizationID, &t.PoolID, &t.Name, &t.Issuer, &t.JWKSURI, &audience, &subjects, &status, &t.CreatedAt, &t.UpdatedAt, &t.Revision); err != nil {
		return nil, err
	}
	if err := unmarshalStringArray(audience, &t.Audience); err != nil {
		return nil, err
	}
	if err := unmarshalStringArray(subjects, &t.Subjects); err != nil {
		return nil, err
	}
	t.Status = domain.FederationTrustStatus(status)
	return &t, nil
}

func (r Repository) SetTrustStatus(ctx context.Context, id, orgID string, newStatus domain.FederationTrustStatus) error {
	if newStatus != domain.TrustStatusActive && newStatus != domain.TrustStatusDisabled {
		return apperrors.NewInvalidArgument("trust status must be active or disabled")
	}
	result, err := r.db.ExecContext(ctx, qUpdateTrustStatus, string(newStatus), id, orgID)
	if err != nil {
		return err
	}
	return checkAffected(result, "federation trust status conflict")
}

func (r Repository) DeleteTrust(ctx context.Context, id, orgID string) error {
	result, err := r.db.ExecContext(ctx, qDeleteTrust, id, orgID)
	if err != nil {
		return err
	}
	return checkAffected(result, "federation trust delete conflict")
}

func (r Repository) ListAccounts(ctx context.Context, q domain.ListAccountsQuery) ([]*domain.ServiceAccount, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, qCountAccounts, q.OrganizationID, q.PoolID).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.QueryContext(ctx, qListAccounts, q.OrganizationID, q.PoolID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []*domain.ServiceAccount{}
	for rows.Next() {
		a, err := scanAccountRow(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, a)
	}
	return items, total, rows.Err()
}

func (r Repository) GetAccount(ctx context.Context, id, orgID string) (*domain.ServiceAccount, error) {
	return scanAccountRow(r.db.QueryRowContext(ctx, qGetAccount, id, orgID))
}

func (r Repository) GetAccountByClientID(ctx context.Context, clientID string) (*domain.ServiceAccount, error) {
	return scanAccountRow(r.db.QueryRowContext(ctx, qGetAccountByClientID, clientID))
}

func (r Repository) GetAccountByName(ctx context.Context, orgID, name string) (*domain.ServiceAccount, error) {
	return scanAccountRow(r.db.QueryRowContext(ctx, qGetAccountByName, orgID, name))
}

func (r Repository) CreateAccount(ctx context.Context, a *domain.ServiceAccount) error {
	scopes, err := marshalStringArray(a.Scopes)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, qInsertAccount, a.ID, a.OrganizationID, a.PoolID, a.Name, a.Description, a.ClientID, a.ClientSecretHash, scopes, a.AccessTokenTTLSeconds)
	return err
}

func scanAccountRow(s scannable) (*domain.ServiceAccount, error) {
	var a domain.ServiceAccount
	var scopes, status string
	if err := s.Scan(&a.ID, &a.OrganizationID, &a.PoolID, &a.Name, &a.Description, &a.ClientID, &a.ClientSecretHash, &scopes, &a.AccessTokenTTLSeconds, &a.LastRotatedAt, &status, &a.CreatedAt, &a.UpdatedAt, &a.Revision); err != nil {
		return nil, err
	}
	if err := unmarshalStringArray(scopes, &a.Scopes); err != nil {
		return nil, err
	}
	a.Status = domain.ServiceAccountStatus(status)
	return &a, nil
}

func (r Repository) SetAccountStatus(ctx context.Context, id, orgID string, newStatus domain.ServiceAccountStatus, expectedRevision int64) error {
	if newStatus != domain.SAStatusActive && newStatus != domain.SAStatusDisabled {
		return apperrors.NewInvalidArgument("service account status must be active or disabled")
	}
	result, err := r.db.ExecContext(ctx, qUpdateAccountStatus, string(newStatus), id, orgID, expectedRevision)
	if err != nil {
		return err
	}
	return checkAffected(result, "service account revision conflict")
}

func (r Repository) RotateAccountSecret(ctx context.Context, id, orgID, secretHash string, rotatedAt time.Time, expectedRevision int64) error {
	result, err := r.db.ExecContext(ctx, qRotateAccountSecret, secretHash, rotatedAt, id, orgID, expectedRevision)
	if err != nil {
		return err
	}
	return checkAffected(result, "service account revision conflict")
}

func (r Repository) DeleteAccount(ctx context.Context, id, orgID string, expectedRevision int64) error {
	result, err := r.db.ExecContext(ctx, qDeleteAccount, id, orgID, expectedRevision)
	if err != nil {
		return err
	}
	return checkAffected(result, "service account revision conflict")
}

func marshalStringArray(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	encoded, err := json.Marshal(values)
	return string(encoded), err
}

func unmarshalStringArray(value string, target *[]string) error {
	if value == "" {
		*target = []string{}
		return nil
	}
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return err
	}
	if *target == nil {
		*target = []string{}
	}
	return nil
}

func checkAffected(result sql.Result, conflictMessage string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict(conflictMessage)
	}
	return nil
}
