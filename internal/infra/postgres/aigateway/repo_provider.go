package aigateway

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// ============================================================================
// ProvidersRepo
// ============================================================================

type ProvidersRepository struct {
	db shared.DBTX
}

func NewProvidersRepository(db shared.DBTX) ProvidersRepository {
	return ProvidersRepository{db: db}
}

var _ domain.ProvidersRepo = ProvidersRepository{}
var _ domain.TenantScopedProvidersRepo = ProvidersRepository{}

func (r ProvidersRepository) GetByID(ctx context.Context, id string) (*domain.LlmProvider, error) {
	row := r.db.QueryRow(ctx, queryProviderGetByID, id)
	p, err := scanLlmProvider(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("provider not found")
		}
		return nil, err
	}
	return p, nil
}

// GetByIDForOrganization is the management by-ID read. The trusted tenant is
// an explicit predicate, so a foreign or missing id is NotFound.
func (r ProvidersRepository) GetByIDForOrganization(ctx context.Context, organizationID, id string) (*domain.LlmProvider, error) {
	row := r.db.QueryRow(ctx, queryProviderGetByIDForOrganization, id, organizationID)
	p, err := scanLlmProvider(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("provider not found")
		}
		return nil, err
	}
	return p, nil
}

func (r ProvidersRepository) GetByName(ctx context.Context, organizationID, name string) (*domain.LlmProvider, error) {
	row := r.db.QueryRow(ctx, queryProviderGetByName, organizationID, name)
	p, err := scanLlmProvider(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("provider not found")
		}
		return nil, err
	}
	return p, nil
}

func (r ProvidersRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmProvider, error) {
	rows, err := r.db.Query(ctx, queryProviderList, organizationID)
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

func (r ProvidersRepository) Create(ctx context.Context, p *domain.LlmProvider) error {
	protocols, err := shared.MarshalJSON(p.SupportedProtocols)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryProviderInsert,
		p.ID, p.OrganizationID, p.ID, p.Name, p.DisplayName, p.ProviderType,
		protocols, p.PreferredProtocol, p.BaseURL,
		p.APIKeySealed, p.APIKeyFingerprint, p.Status,
		p.RequestTimeoutMs, p.MaxRetries, p.Priority, p.Weight,
		p.RPMLimit, p.TPMLimit,
		p.CreatedAt, p.UpdatedAt, p.DeletedAt, p.Revision,
	)
	return err
}

func (r ProvidersRepository) Update(ctx context.Context, p *domain.LlmProvider) error {
	protocols, err := shared.MarshalJSON(p.SupportedProtocols)
	if err != nil {
		return err
	}
	var newRevision int64
	err = r.db.QueryRow(ctx, queryProviderUpdate,
		p.Name, p.DisplayName, p.ProviderType, protocols, p.PreferredProtocol,
		p.BaseURL, p.APIKeySealed, p.APIKeyFingerprint, p.Status,
		p.RequestTimeoutMs, p.MaxRetries, p.Priority, p.Weight,
		p.RPMLimit, p.TPMLimit,
		p.UpdatedAt, p.DeletedAt,
		p.Revision-1, p.ID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("provider revision conflict")
		}
		return err
	}
	p.Revision = newRevision
	return nil
}

func (r ProvidersRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, queryProviderDelete, id)
	return err
}

// UpdateForOrganization persists the aggregate only inside organizationID.
// A concurrent ownership/revision change surfaces as the same revision
// conflict as a stale write, never as a cross-tenant update.
func (r ProvidersRepository) UpdateForOrganization(ctx context.Context, p *domain.LlmProvider, organizationID string) error {
	protocols, err := shared.MarshalJSON(p.SupportedProtocols)
	if err != nil {
		return err
	}
	var newRevision int64
	err = r.db.QueryRow(ctx, queryProviderUpdateForOrganization,
		p.Name, p.DisplayName, p.ProviderType, protocols, p.PreferredProtocol,
		p.BaseURL, p.APIKeySealed, p.APIKeyFingerprint, p.Status,
		p.RequestTimeoutMs, p.MaxRetries, p.Priority, p.Weight,
		p.RPMLimit, p.TPMLimit,
		p.UpdatedAt, p.DeletedAt,
		p.Revision-1, p.ID, organizationID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("provider revision conflict")
		}
		return err
	}
	p.Revision = newRevision
	return nil
}

// DeleteForOrganization soft-deletes only a row owned by organizationID. A
// foreign or already deleted id affects no rows and is reported as NotFound.
func (r ProvidersRepository) DeleteForOrganization(ctx context.Context, organizationID, id string) error {
	tag, err := r.db.Exec(ctx, queryProviderDeleteForOrganization, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("provider not found")
	}
	return nil
}
