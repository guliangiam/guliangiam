package adapters

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type adapterRow struct {
	ID, OrganizationID, Name, DisplayName, Description, Type, Server, Database, TableName, Username, Password string
	// Phase 2: Extended fields
	Owner                                    string
	UseSameDb                                bool
	DatabaseType                             string
	Host                                     string
	Port                                     *int
	DbUser, DbPassword, DatabaseName, Policy string
	IsEnabled                                bool
	CreatedAt, UpdatedAt                     time.Time
	Revision                                 int64
}

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

type scannable interface {
	Scan(dest ...any) error
}

func scanAdapterRow(s scannable) (adapterRow, error) {
	var row adapterRow
	err := s.Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Description,
		&row.Type, &row.Server, &row.Database, &row.TableName, &row.Username, &row.Password,
		&row.Owner, &row.UseSameDb, &row.DatabaseType, &row.Host, &row.Port,
		&row.DbUser, &row.DbPassword, &row.DatabaseName, &row.Policy, &row.IsEnabled,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	return row, err
}

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Adapter, error) {
	row, err := scanAdapterRow(r.db.QueryRow(ctx, queryGetAdapterByID, id, organizationID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("adapter not found")
		}
		return nil, err
	}
	return rowToDomain(row)
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Adapter, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListAdapters, query.OrganizationID, query.Keyword, query.IsEnabled, query.DatabaseType, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Adapter, 0)
	for rows.Next() {
		row, err := scanAdapterRow(rows)
		if err != nil {
			return nil, 0, err
		}
		item, err := rowToDomain(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountAdapters, query.OrganizationID, query.Keyword, query.IsEnabled, query.DatabaseType).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, a *domain.Adapter) error {
	password, dbPassword, err := adapterSecretsForWrite(a)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryInsertAdapter,
		a.ID, a.OrganizationID, a.Name, a.DisplayName, a.Description,
		a.Type, a.Server, a.Database, a.TableName, a.Username, password,
		a.Owner, a.UseSameDb, a.DatabaseType, a.Host, a.Port,
		a.DbUser, dbPassword, a.DatabaseName, a.Policy, a.IsEnabled,
		a.CreatedAt, a.UpdatedAt, a.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, a *domain.Adapter) error {
	password, dbPassword, err := adapterSecretsForWrite(a)
	if err != nil {
		return err
	}
	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateAdapter,
		a.DisplayName, a.Description, a.Type, a.Server, a.Database, a.TableName,
		a.Username, password,
		a.Owner, a.UseSameDb, a.DatabaseType, a.Host, a.Port,
		a.DbUser, dbPassword, a.DatabaseName, a.Policy, a.IsEnabled,
		a.UpdatedAt, a.Revision-1, a.ID, a.OrganizationID).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("adapter revision conflict")
		}
		return err
	}
	a.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteAdapter, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("adapter not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

// adapterEncryptedMapper guards the DB-058 confidential columns: password and
// db_password are stored as AES-256-GCM ciphertext.
var adapterEncryptedMapper = shared.NewEncryptedFieldMapper()

const (
	adapterTable            = "guliangiam.adapters"
	adapterColumnPassword   = "password"
	adapterColumnDbPassword = "db_password"
)

func adapterAAD(adapterID, column string) crypto.AADContext {
	return crypto.AADContext{Table: adapterTable, Column: column, PKValue: adapterID}
}

// adapterSecretsForWrite returns the values to persist for the confidential
// columns without mutating the domain object.
func adapterSecretsForWrite(a *domain.Adapter) (password, dbPassword string, err error) {
	password, dbPassword = a.Password, a.DbPassword
	if !adapterEncryptedMapper.IsEnabled() {
		return password, dbPassword, nil
	}
	password, err = adapterEncryptedMapper.EncryptColumn(adapterAAD(a.ID, adapterColumnPassword), a.Password)
	if err != nil {
		return "", "", apperrors.WrapInternalf(err, "encrypt adapter password")
	}
	dbPassword, err = adapterEncryptedMapper.EncryptColumn(adapterAAD(a.ID, adapterColumnDbPassword), a.DbPassword)
	if err != nil {
		return "", "", apperrors.WrapInternalf(err, "encrypt adapter db_password")
	}
	return password, dbPassword, nil
}

func rowToDomain(row adapterRow) (*domain.Adapter, error) {
	a := &domain.Adapter{
		ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name,
		DisplayName: row.DisplayName, Description: row.Description,
		Type: row.Type, Server: row.Server, Database: row.Database,
		TableName: row.TableName, Username: row.Username, Password: row.Password,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision,
		// Phase 2: Extended fields
		Owner: row.Owner, UseSameDb: row.UseSameDb, DatabaseType: row.DatabaseType,
		Host: row.Host, Port: row.Port, DbUser: row.DbUser, DbPassword: row.DbPassword,
		DatabaseName: row.DatabaseName, Policy: row.Policy, IsEnabled: row.IsEnabled,
	}
	if !adapterEncryptedMapper.IsEnabled() {
		return a, nil
	}
	password, err := adapterEncryptedMapper.DecryptColumn(adapterAAD(a.ID, adapterColumnPassword), a.Password)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decrypt adapter password")
	}
	dbPassword, err := adapterEncryptedMapper.DecryptColumn(adapterAAD(a.ID, adapterColumnDbPassword), a.DbPassword)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decrypt adapter db_password")
	}
	a.Password = password
	a.DbPassword = dbPassword
	return a, nil
}

func maxInt(v, f int) int {
	if v <= 0 {
		return f
	}
	return v
}
