package workload

import (
	"context"
	"fmt"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/workload"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

var _ domain.Repository = Repository{}

type scannable interface {
	Scan(dest ...any) error
}

func (r Repository) ListPools(ctx context.Context, q domain.ListPoolsQuery) ([]*domain.Pool, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM guliangiam.identity_pools WHERE organization_id = $1", q.OrganizationID).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := shared.ClampPageSize(q.PageSize)
	rows, err := r.db.Query(ctx, qListPools+fmt.Sprintf(" LIMIT %d OFFSET %d", limit, shared.PageOffset(q.Page, q.PageSize)), q.OrganizationID)
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
	if err := r.db.QueryRow(ctx, qGetPool, id, orgID).Scan(&p.ID, &p.OrganizationID, &p.Name, &p.DisplayName, &p.Description, &p.CreatedAt, &p.UpdatedAt, &p.Revision); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r Repository) CreatePool(ctx context.Context, p *domain.Pool) error {
	_, err := r.db.Exec(ctx, qInsertPool, p.ID, p.OrganizationID, p.Name, p.DisplayName, p.Description)
	return err
}

func (r Repository) ListTrusts(ctx context.Context, q domain.ListTrustsQuery) ([]*domain.FederationTrust, int64, error) {
	args := []any{q.OrganizationID, q.PoolID}
	var total int64
	if err := r.db.QueryRow(ctx, qCountTrusts, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := shared.ClampPageSize(q.PageSize)
	statement := qListTrusts + fmt.Sprintf(" LIMIT %d OFFSET %d", limit, shared.PageOffset(q.Page, q.PageSize))
	rows, err := r.db.Query(ctx, statement, args...)
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
	return scanTrustRow(r.db.QueryRow(ctx, qGetTrust, id, orgID))
}

func (r Repository) FindActiveTrustsByIssuer(ctx context.Context, issuer string) ([]*domain.FederationTrust, error) {
	rows, err := r.db.Query(ctx, qFindActiveTrustsByIssuer, issuer)
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
	_, err := r.db.Exec(ctx, qInsertTrust, t.ID, t.OrganizationID, t.PoolID, t.Name, t.Issuer, t.JWKSURI, t.Audience, t.Subjects)
	return err
}

func scanTrustRow(s scannable) (*domain.FederationTrust, error) {
	var t domain.FederationTrust
	var status string
	if err := s.Scan(&t.ID, &t.OrganizationID, &t.PoolID, &t.Name, &t.Issuer, &t.JWKSURI, &t.Audience, &t.Subjects, &status, &t.CreatedAt, &t.UpdatedAt, &t.Revision); err != nil {
		return nil, err
	}
	t.Status = domain.FederationTrustStatus(status)
	return &t, nil
}

func (r Repository) SetTrustStatus(ctx context.Context, id, orgID string, newStatus domain.FederationTrustStatus) error {
	if newStatus != domain.TrustStatusActive && newStatus != domain.TrustStatusDisabled {
		return apperrors.NewInvalidArgument("trust status must be active or disabled")
	}
	result, err := r.db.Exec(ctx, qUpdateTrustStatus, string(newStatus), id, orgID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewConflict("federation trust status conflict")
	}
	return nil
}

func (r Repository) DeleteTrust(ctx context.Context, id, orgID string) error {
	result, err := r.db.Exec(ctx, qDeleteTrust, id, orgID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewConflict("federation trust delete conflict")
	}
	return nil
}

func (r Repository) ListAccounts(ctx context.Context, q domain.ListAccountsQuery) ([]*domain.ServiceAccount, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, qCountAccounts, q.OrganizationID, q.PoolID).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := shared.ClampPageSize(q.PageSize)
	rows, err := r.db.Query(ctx, qListAccounts+fmt.Sprintf(" LIMIT %d OFFSET %d", limit, shared.PageOffset(q.Page, q.PageSize)), q.OrganizationID, q.PoolID)
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
	return scanAccountRow(r.db.QueryRow(ctx, qGetAccount, id, orgID))
}

func (r Repository) GetAccountByClientID(ctx context.Context, clientID string) (*domain.ServiceAccount, error) {
	return scanAccountRow(r.db.QueryRow(ctx, qGetAccountByClientID, clientID))
}

func (r Repository) GetAccountByName(ctx context.Context, orgID, name string) (*domain.ServiceAccount, error) {
	return scanAccountRow(r.db.QueryRow(ctx, qGetAccountByName, orgID, name))
}

func (r Repository) CreateAccount(ctx context.Context, a *domain.ServiceAccount) error {
	_, err := r.db.Exec(ctx, qInsertAccount, a.ID, a.OrganizationID, a.PoolID, a.Name, a.Description, a.ClientID, a.ClientSecretHash, a.Scopes, a.AccessTokenTTLSeconds)
	return err
}

func scanAccountRow(s scannable) (*domain.ServiceAccount, error) {
	var a domain.ServiceAccount
	var status string
	if err := s.Scan(&a.ID, &a.OrganizationID, &a.PoolID, &a.Name, &a.Description, &a.ClientID, &a.ClientSecretHash, &a.Scopes, &a.AccessTokenTTLSeconds, &a.LastRotatedAt, &status, &a.CreatedAt, &a.UpdatedAt, &a.Revision); err != nil {
		return nil, err
	}
	a.Status = domain.ServiceAccountStatus(status)
	return &a, nil
}

func (r Repository) SetAccountStatus(ctx context.Context, id, orgID string, newStatus domain.ServiceAccountStatus, expectedRevision int64) error {
	if newStatus != domain.SAStatusActive && newStatus != domain.SAStatusDisabled {
		return apperrors.NewInvalidArgument("service account status must be active or disabled")
	}
	result, err := r.db.Exec(ctx, qUpdateAccountStatus, string(newStatus), id, orgID, expectedRevision)
	if err != nil {
		return err
	}
	return shared.CheckAffectedRows(result.RowsAffected())
}

func (r Repository) RotateAccountSecret(ctx context.Context, id, orgID, secretHash string, rotatedAt time.Time, expectedRevision int64) error {
	result, err := r.db.Exec(ctx, qRotateAccountSecret, secretHash, rotatedAt, id, orgID, expectedRevision)
	if err != nil {
		return err
	}
	return shared.CheckAffectedRows(result.RowsAffected())
}

func (r Repository) DeleteAccount(ctx context.Context, id, orgID string, expectedRevision int64) error {
	result, err := r.db.Exec(ctx, qDeleteAccount, id, orgID, expectedRevision)
	if err != nil {
		return err
	}
	return shared.CheckAffectedRows(result.RowsAffected())
}
