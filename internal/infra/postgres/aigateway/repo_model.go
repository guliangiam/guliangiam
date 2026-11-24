package aigateway

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// ============================================================================
// ModelsRepo
// ============================================================================

type ModelsRepository struct {
	db shared.DBTX
}

func NewModelsRepository(db shared.DBTX) ModelsRepository {
	return ModelsRepository{db: db}
}

var _ domain.ModelsRepo = ModelsRepository{}
var _ domain.TenantScopedModelsRepo = ModelsRepository{}

func (r ModelsRepository) GetByID(ctx context.Context, id string) (*domain.LlmModel, error) {
	row := r.db.QueryRow(ctx, queryModelGetByID, id)
	m, err := scanLlmModel(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("model not found")
		}
		return nil, err
	}
	return m, nil
}

// GetByIDForOrganization is the management by-ID read. The trusted tenant is
// an explicit predicate, so a foreign or missing id is NotFound.
func (r ModelsRepository) GetByIDForOrganization(ctx context.Context, organizationID, id string) (*domain.LlmModel, error) {
	row := r.db.QueryRow(ctx, queryModelGetByIDForOrganization, id, organizationID)
	m, err := scanLlmModel(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("model not found")
		}
		return nil, err
	}
	return m, nil
}

func (r ModelsRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmModel, error) {
	rows, err := r.db.Query(ctx, queryModelList, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := make([]*domain.LlmModel, 0, 32)
	for rows.Next() {
		m, err := scanLlmModel(rows)
		if err != nil {
			return nil, err
		}
		models = append(models, m)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return models, nil
}

func (r ModelsRepository) ListByProvider(ctx context.Context, providerID string) ([]*domain.LlmModel, error) {
	rows, err := r.db.Query(ctx, queryModelListByProvider, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := make([]*domain.LlmModel, 0, 32)
	for rows.Next() {
		m, err := scanLlmModel(rows)
		if err != nil {
			return nil, err
		}
		models = append(models, m)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return models, nil
}

func (r ModelsRepository) Create(ctx context.Context, m *domain.LlmModel) error {
	caps, err := shared.MarshalJSON(m.Capabilities)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryModelInsert,
		m.ID, m.OrganizationID, m.ProviderID, m.ProviderModelName, m.DisplayName,
		caps, m.MaxContextTokens, m.MaxOutputTokens,
		m.Status, m.CreatedAt, m.UpdatedAt, m.DeletedAt, m.Revision,
	)
	return err
}

func (r ModelsRepository) Update(ctx context.Context, m *domain.LlmModel) error {
	caps, err := shared.MarshalJSON(m.Capabilities)
	if err != nil {
		return err
	}
	var newRevision int64
	err = r.db.QueryRow(ctx, queryModelUpdate,
		m.ProviderModelName, m.DisplayName, caps,
		m.MaxContextTokens, m.MaxOutputTokens, m.Status,
		m.UpdatedAt, m.DeletedAt,
		m.Revision-1, m.ID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("model revision conflict")
		}
		return err
	}
	m.Revision = newRevision
	return nil
}

func (r ModelsRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, queryModelDelete, id)
	return err
}

// UpdateForOrganization persists the aggregate only inside organizationID.
// A concurrent ownership/revision change surfaces as the same revision
// conflict as a stale write, never as a cross-tenant update.
func (r ModelsRepository) UpdateForOrganization(ctx context.Context, m *domain.LlmModel, organizationID string) error {
	caps, err := shared.MarshalJSON(m.Capabilities)
	if err != nil {
		return err
	}
	var newRevision int64
	err = r.db.QueryRow(ctx, queryModelUpdateForOrganization,
		m.ProviderModelName, m.DisplayName, caps,
		m.MaxContextTokens, m.MaxOutputTokens, m.Status,
		m.UpdatedAt, m.DeletedAt,
		m.Revision-1, m.ID, organizationID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("model revision conflict")
		}
		return err
	}
	m.Revision = newRevision
	return nil
}

// DeleteForOrganization soft-deletes only a row owned by organizationID. A
// foreign or already deleted id affects no rows and is reported as NotFound.
func (r ModelsRepository) DeleteForOrganization(ctx context.Context, organizationID, id string) error {
	tag, err := r.db.Exec(ctx, queryModelDeleteForOrganization, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("model not found")
	}
	return nil
}

// ============================================================================
// AliasesRepo
// ============================================================================

type AliasesRepository struct {
	db shared.DBTX
}

func NewAliasesRepository(db shared.DBTX) AliasesRepository {
	return AliasesRepository{db: db}
}

var _ domain.AliasesRepo = AliasesRepository{}
var _ domain.TenantScopedAliasesRepo = AliasesRepository{}

func (r AliasesRepository) GetByID(ctx context.Context, id string) (*domain.LlmModelAlias, error) {
	row := r.db.QueryRow(ctx, queryAliasGetByID, id)
	a, err := scanLlmModelAlias(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("alias not found")
		}
		return nil, err
	}
	return a, nil
}

// GetByIDForOrganization is the management by-ID read. The trusted tenant is
// an explicit predicate, so a foreign or missing id is NotFound.
func (r AliasesRepository) GetByIDForOrganization(ctx context.Context, organizationID, id string) (*domain.LlmModelAlias, error) {
	row := r.db.QueryRow(ctx, queryAliasGetByIDForOrganization, id, organizationID)
	a, err := scanLlmModelAlias(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("alias not found")
		}
		return nil, err
	}
	return a, nil
}

func (r AliasesRepository) GetByName(ctx context.Context, organizationID, alias string) (*domain.LlmModelAlias, error) {
	row := r.db.QueryRow(ctx, queryAliasGetByName, organizationID, alias)
	a, err := scanLlmModelAlias(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("alias not found")
		}
		return nil, err
	}
	return a, nil
}

func (r AliasesRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmModelAlias, error) {
	rows, err := r.db.Query(ctx, queryAliasList, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	aliases := make([]*domain.LlmModelAlias, 0, 16)
	for rows.Next() {
		a, err := scanLlmModelAlias(rows)
		if err != nil {
			return nil, err
		}
		aliases = append(aliases, a)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return aliases, nil
}

func (r AliasesRepository) ListAll(ctx context.Context) ([]*domain.LlmModelAlias, error) {
	rows, err := r.db.Query(ctx, queryAliasListAll)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	aliases := make([]*domain.LlmModelAlias, 0, 32)
	for rows.Next() {
		a, err := scanLlmModelAlias(rows)
		if err != nil {
			return nil, err
		}
		aliases = append(aliases, a)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return aliases, nil
}

func (r AliasesRepository) Create(ctx context.Context, a *domain.LlmModelAlias) error {
	_, err := r.db.Exec(ctx, queryAliasInsert,
		a.ID, a.OrganizationID, a.Alias, a.DisplayName,
		a.Status, a.CreatedAt, a.UpdatedAt, a.DeletedAt, a.Revision,
	)
	return err
}

func (r AliasesRepository) Update(ctx context.Context, a *domain.LlmModelAlias) error {
	var newRevision int64
	err := r.db.QueryRow(ctx, queryAliasUpdate,
		a.Alias, a.DisplayName, a.Status,
		a.UpdatedAt, a.DeletedAt,
		a.Revision-1, a.ID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("alias revision conflict")
		}
		return err
	}
	a.Revision = newRevision
	return nil
}

func (r AliasesRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, queryAliasDelete, id)
	return err
}

// UpdateForOrganization persists the aggregate only inside organizationID.
// A concurrent ownership/revision change surfaces as the same revision
// conflict as a stale write, never as a cross-tenant update.
func (r AliasesRepository) UpdateForOrganization(ctx context.Context, a *domain.LlmModelAlias, organizationID string) error {
	var newRevision int64
	err := r.db.QueryRow(ctx, queryAliasUpdateForOrganization,
		a.Alias, a.DisplayName, a.Status,
		a.UpdatedAt, a.DeletedAt,
		a.Revision-1, a.ID, organizationID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("alias revision conflict")
		}
		return err
	}
	a.Revision = newRevision
	return nil
}

// DeleteForOrganization soft-deletes only a row owned by organizationID. A
// foreign or already deleted id affects no rows and is reported as NotFound.
func (r AliasesRepository) DeleteForOrganization(ctx context.Context, organizationID, id string) error {
	tag, err := r.db.Exec(ctx, queryAliasDeleteForOrganization, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("alias not found")
	}
	return nil
}

// ============================================================================
// PriceVersionsRepo
// ============================================================================

type PriceVersionsRepository struct {
	db shared.DBTX
}

func NewPriceVersionsRepository(db shared.DBTX) PriceVersionsRepository {
	return PriceVersionsRepository{db: db}
}

var _ domain.PriceVersionsRepo = PriceVersionsRepository{}
var _ domain.TenantScopedPriceVersionsRepo = PriceVersionsRepository{}

func (r PriceVersionsRepository) GetByID(ctx context.Context, id string) (*domain.LlmPriceVersion, error) {
	row := r.db.QueryRow(ctx, queryPriceVersionGetByID, id)
	v, err := scanLlmPriceVersion(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("price version not found")
		}
		return nil, err
	}
	return v, nil
}

// GetByIDForOrganization is the management by-ID read. The trusted tenant is
// an explicit predicate, so a foreign or missing id is NotFound.
func (r PriceVersionsRepository) GetByIDForOrganization(ctx context.Context, organizationID, id string) (*domain.LlmPriceVersion, error) {
	row := r.db.QueryRow(ctx, queryPriceVersionGetByIDForOrganization, id, organizationID)
	v, err := scanLlmPriceVersion(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("price version not found")
		}
		return nil, err
	}
	return v, nil
}

func (r PriceVersionsRepository) GetByModel(ctx context.Context, modelID string) (*domain.LlmPriceVersion, error) {
	row := r.db.QueryRow(ctx, queryPriceVersionGetByModel, modelID)
	v, err := scanLlmPriceVersion(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("price version not found")
		}
		return nil, err
	}
	return v, nil
}

func (r PriceVersionsRepository) ListByModel(ctx context.Context, modelID string) ([]*domain.LlmPriceVersion, error) {
	rows, err := r.db.Query(ctx, queryPriceVersionListByModel, modelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := make([]*domain.LlmPriceVersion, 0, 8)
	for rows.Next() {
		v, err := scanLlmPriceVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return versions, nil
}

func (r PriceVersionsRepository) Create(ctx context.Context, v *domain.LlmPriceVersion) error {
	_, err := r.db.Exec(ctx, queryPriceVersionInsert,
		v.ID, v.OrganizationID, v.ModelID, v.VersionNo,
		v.InputPricePer1M, v.OutputPricePer1M, v.Currency, v.EffectiveFrom,
		v.CreatedAt, v.UpdatedAt, v.DeletedAt, v.Revision,
	)
	return err
}

func (r PriceVersionsRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmPriceVersion, error) {
	rows, err := r.db.Query(ctx, queryPriceVersionList, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*domain.LlmPriceVersion, 0, 16)
	for rows.Next() {
		v, err := scanLlmPriceVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r PriceVersionsRepository) Update(ctx context.Context, v *domain.LlmPriceVersion) error {
	tag, err := r.db.Exec(ctx, queryPriceVersionUpdate,
		v.VersionNo, v.InputPricePer1M, v.OutputPricePer1M,
		v.Currency, v.EffectiveFrom, v.UpdatedAt, v.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("price version not found")
	}
	return nil
}

func (r PriceVersionsRepository) Delete(ctx context.Context, id string) error {
	now := time.Now().UTC()
	tag, err := r.db.Exec(ctx, queryPriceVersionSoftDelete, now, now, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("price version not found")
	}
	return nil
}

// UpdateForOrganization persists the aggregate only inside organizationID.
// The current revision is the guard; a concurrent ownership/revision change
// surfaces as the same conflict as a stale write, never as a cross-tenant update.
func (r PriceVersionsRepository) UpdateForOrganization(ctx context.Context, v *domain.LlmPriceVersion, organizationID string) error {
	var newRevision int64
	err := r.db.QueryRow(ctx, queryPriceVersionUpdateForOrganization,
		v.VersionNo, v.InputPricePer1M, v.OutputPricePer1M,
		v.Currency, v.EffectiveFrom, v.UpdatedAt,
		v.Revision, v.ID, organizationID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("price version revision conflict")
		}
		return err
	}
	v.Revision = newRevision
	return nil
}

// DeleteForOrganization soft-deletes only a row owned by organizationID. A
// foreign or already deleted id affects no rows and is reported as NotFound.
func (r PriceVersionsRepository) DeleteForOrganization(ctx context.Context, organizationID, id string) error {
	now := time.Now().UTC()
	tag, err := r.db.Exec(ctx, queryPriceVersionDeleteForOrganization, now, now, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("price version not found")
	}
	return nil
}
