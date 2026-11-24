package aigateway

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ============================================================================
// ProvidersRepo
// ============================================================================

type ProvidersRepository struct {
	db mysqlshared.DBTX
}

func NewProvidersRepository(db mysqlshared.DBTX) ProvidersRepository {
	return ProvidersRepository{db: db}
}

var _ domain.ProvidersRepo = (*ProvidersRepository)(nil)
var _ domain.TenantScopedProvidersRepo = (*ProvidersRepository)(nil)

func (r *ProvidersRepository) GetByID(ctx context.Context, id string) (*domain.LlmProvider, error) {
	row := r.db.QueryRowContext(ctx, queryProviderGetByID, id)
	p, err := scanLlmProvider(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("provider not found")
		}
		return nil, err
	}
	return p, nil
}

// GetByIDForOrganization is the management by-ID read. MySQL has no RLS, so
// the explicit organization_id predicate is the only tenant boundary.
func (r *ProvidersRepository) GetByIDForOrganization(ctx context.Context, organizationID, id string) (*domain.LlmProvider, error) {
	row := r.db.QueryRowContext(ctx, queryProviderGetByIDForOrganization, id, organizationID)
	p, err := scanLlmProvider(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("provider not found")
		}
		return nil, err
	}
	return p, nil
}

func (r *ProvidersRepository) GetByName(ctx context.Context, organizationID, name string) (*domain.LlmProvider, error) {
	row := r.db.QueryRowContext(ctx, queryProviderGetByName, organizationID, name)
	p, err := scanLlmProvider(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("provider not found")
		}
		return nil, err
	}
	return p, nil
}

func (r *ProvidersRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmProvider, error) {
	rows, err := r.db.QueryContext(ctx, queryProviderList, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	providers := make([]*domain.LlmProvider, 0, 16)
	for rows.Next() {
		p, err := scanLlmProvider(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return providers, nil
}

func (r *ProvidersRepository) Create(ctx context.Context, p *domain.LlmProvider) error {
	protocols, err := json.Marshal(p.SupportedProtocols)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryProviderInsert,
		p.ID, p.OrganizationID, p.ID, p.Name, p.DisplayName, p.ProviderType,
		string(protocols), p.PreferredProtocol, p.BaseURL,
		p.APIKeySealed, p.APIKeyFingerprint, p.Status,
		p.RequestTimeoutMs, p.MaxRetries, p.Priority, p.Weight,
		p.RPMLimit, p.TPMLimit,
		p.CreatedAt, p.UpdatedAt, p.DeletedAt, p.Revision,
	)
	return err
}

func (r *ProvidersRepository) Update(ctx context.Context, p *domain.LlmProvider) error {
	protocols, err := json.Marshal(p.SupportedProtocols)
	if err != nil {
		return err
	}
	// MySQL 无 RETURNING：用 ExecContext + RowsAffected 判断 revision 冲突
	res, err := r.db.ExecContext(ctx, queryProviderUpdate,
		p.Name, p.DisplayName, p.ProviderType, string(protocols), p.PreferredProtocol,
		p.BaseURL, p.APIKeySealed, p.APIKeyFingerprint, p.Status,
		p.RequestTimeoutMs, p.MaxRetries, p.Priority, p.Weight,
		p.RPMLimit, p.TPMLimit,
		p.UpdatedAt, p.DeletedAt,
		p.ID, p.Revision,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewConflictf("provider revision conflict (id=%s)", p.ID)
	}
	p.Revision++
	return nil
}

func (r *ProvidersRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, queryProviderDelete, id)
	return err
}

// UpdateForOrganization persists the aggregate only inside organizationID; a
// concurrent ownership or revision change is reported as a revision conflict.
func (r *ProvidersRepository) UpdateForOrganization(ctx context.Context, p *domain.LlmProvider, organizationID string) error {
	protocols, err := json.Marshal(p.SupportedProtocols)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, queryProviderUpdateForOrganization,
		p.Name, p.DisplayName, p.ProviderType, string(protocols), p.PreferredProtocol,
		p.BaseURL, p.APIKeySealed, p.APIKeyFingerprint, p.Status,
		p.RequestTimeoutMs, p.MaxRetries, p.Priority, p.Weight,
		p.RPMLimit, p.TPMLimit,
		p.UpdatedAt, p.DeletedAt,
		p.ID, p.Revision, organizationID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewConflictf("provider revision conflict (id=%s)", p.ID)
	}
	p.Revision++
	return nil
}

// DeleteForOrganization soft-deletes only a row owned by organizationID. A
// foreign or already deleted id affects no rows and is reported as NotFound.
func (r *ProvidersRepository) DeleteForOrganization(ctx context.Context, organizationID, id string) error {
	res, err := r.db.ExecContext(ctx, queryProviderDeleteForOrganization, id, organizationID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewNotFound("provider not found")
	}
	return nil
}
