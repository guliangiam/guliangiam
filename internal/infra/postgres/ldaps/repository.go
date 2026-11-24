package ldaps

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type ldapRow struct {
	ID                  string
	OrganizationID      string
	Name                string
	DisplayName         string
	ServerName          string
	Host                string
	Port                int
	Admin               string
	BindDN              string
	BindPassword        string
	BaseDN              string
	Type                string
	AttributeName       string
	PasswordType        string
	Filter              string
	FilterFields        []string
	EnableSsl           bool
	AllowSelfSignedCert bool
	EnableGroups        bool
	DefaultGroups       []string
	CustomAttributes    []byte
	IsEnabled           bool
	SyncInterval        int
	LastSyncAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Revision            int64
}

func toLdap(row ldapRow) (*domain.Ldap, error) {
	l := &domain.Ldap{
		ID:                  row.ID,
		OrganizationID:      row.OrganizationID,
		Name:                row.Name,
		DisplayName:         row.DisplayName,
		ServerName:          row.ServerName,
		Host:                row.Host,
		Port:                row.Port,
		Admin:               row.Admin,
		BindDN:              row.BindDN,
		BindPassword:        row.BindPassword,
		BaseDN:              row.BaseDN,
		Type:                row.Type,
		AttributeName:       row.AttributeName,
		PasswordType:        row.PasswordType,
		Filter:              row.Filter,
		FilterFields:        row.FilterFields,
		EnableSsl:           row.EnableSsl,
		AllowSelfSignedCert: row.AllowSelfSignedCert,
		EnableGroups:        row.EnableGroups,
		DefaultGroups:       row.DefaultGroups,
		IsEnabled:           row.IsEnabled,
		SyncInterval:        row.SyncInterval,
		LastSyncAt:          row.LastSyncAt,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		Revision:            row.Revision,
	}
	if row.FilterFields == nil {
		l.FilterFields = []string{}
	}
	if row.DefaultGroups == nil {
		l.DefaultGroups = []string{}
	}
	if err := shared.UnmarshalJSON(row.CustomAttributes, &l.CustomAttributes); err != nil {
		return nil, err
	}
	if l.CustomAttributes == nil {
		l.CustomAttributes = map[string]string{}
	}
	if err := decryptLdapSecrets(l); err != nil {
		return nil, err
	}
	return l, nil
}

var ldapEncryptedMapper = shared.NewEncryptedFieldMapper()

const (
	ldapTable              = "guliangiam.ldaps"
	ldapColumnBindPassword = "bind_password"
)

func ldapAAD(ldapID string) crypto.AADContext {
	return crypto.AADContext{Table: ldapTable, Column: ldapColumnBindPassword, PKValue: ldapID}
}

func encryptLdapSecrets(l *domain.Ldap) error {
	if !ldapEncryptedMapper.IsEnabled() {
		return nil
	}
	enc, err := ldapEncryptedMapper.EncryptColumn(ldapAAD(l.ID), l.BindPassword)
	if err != nil {
		return apperrors.WrapInternalf(err, "encrypt bind_password")
	}
	l.BindPassword = enc
	return nil
}

func decryptLdapSecrets(l *domain.Ldap) error {
	if !ldapEncryptedMapper.IsEnabled() {
		return nil
	}
	dec, err := ldapEncryptedMapper.DecryptColumn(ldapAAD(l.ID), l.BindPassword)
	if err != nil {
		return apperrors.WrapInternalf(err, "decrypt bind_password")
	}
	l.BindPassword = dec
	return nil
}

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Ldap, error) {
	return r.getByIDQuery(ctx, queryGetLdapByID, id)
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Ldap, error) {
	return r.getByIDQuery(ctx, queryGetLdapByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Ldap, error) {
	var row ldapRow
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
		&row.ServerName, &row.Host, &row.Port, &row.Admin,
		&row.BindDN, &row.BindPassword, &row.BaseDN,
		&row.Type, &row.AttributeName, &row.PasswordType,
		&row.Filter, &row.FilterFields,
		&row.EnableSsl, &row.AllowSelfSignedCert,
		&row.EnableGroups, &row.DefaultGroups,
		&row.CustomAttributes,
		&row.IsEnabled, &row.SyncInterval, &row.LastSyncAt,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("ldap not found")
		}
		return nil, err
	}

	return toLdap(row)
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Ldap, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListLdaps, query.OrganizationID, query.Keyword, query.IsEnabled, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Ldap, 0)
	for rows.Next() {
		var row ldapRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
			&row.ServerName, &row.Host, &row.Port, &row.Admin,
			&row.BindDN, &row.BindPassword, &row.BaseDN,
			&row.Type, &row.AttributeName, &row.PasswordType,
			&row.Filter, &row.FilterFields,
			&row.EnableSsl, &row.AllowSelfSignedCert,
			&row.EnableGroups, &row.DefaultGroups,
			&row.CustomAttributes,
			&row.IsEnabled, &row.SyncInterval, &row.LastSyncAt,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		l, err := toLdap(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, l)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountLdaps, query.OrganizationID, query.Keyword, query.IsEnabled).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, ldap *domain.Ldap) error {
	customAttrs, err := shared.MarshalJSON(ldap.CustomAttributes)
	if err != nil {
		return err
	}

	if err := encryptLdapSecrets(ldap); err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, queryInsertLdap,
		ldap.ID, ldap.OrganizationID, ldap.Name, ldap.DisplayName,
		ldap.ServerName, ldap.Host, ldap.Port, ldap.Admin,
		ldap.BindDN, ldap.BindPassword, ldap.BaseDN,
		ldap.Type, ldap.AttributeName, ldap.PasswordType, ldap.Filter,
		ldap.FilterFields, ldap.EnableSsl, ldap.AllowSelfSignedCert,
		ldap.EnableGroups, ldap.DefaultGroups, customAttrs,
		ldap.IsEnabled, ldap.SyncInterval, ldap.LastSyncAt,
		ldap.CreatedAt, ldap.UpdatedAt, ldap.Revision,
	)
	return err
}

func (r Repository) Update(ctx context.Context, ldap *domain.Ldap) error {
	customAttrs, err := shared.MarshalJSON(ldap.CustomAttributes)
	if err != nil {
		return err
	}

	if err := encryptLdapSecrets(ldap); err != nil {
		return err
	}

	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateLdap,
		ldap.DisplayName, ldap.ServerName, ldap.Host, ldap.Port,
		ldap.Admin, ldap.BindDN, ldap.BindPassword, ldap.BaseDN,
		ldap.Type, ldap.AttributeName, ldap.PasswordType, ldap.Filter,
		ldap.FilterFields, ldap.EnableSsl, ldap.AllowSelfSignedCert,
		ldap.EnableGroups, ldap.DefaultGroups, customAttrs,
		ldap.IsEnabled, ldap.SyncInterval, ldap.LastSyncAt,
		// App 层已 Revision++，这里要传自增前的值做乐观锁守卫。
		ldap.UpdatedAt, ldap.Revision-1, ldap.ID, ldap.OrganizationID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("ldap revision conflict")
		}
		return err
	}
	ldap.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteLdap, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("ldap not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
