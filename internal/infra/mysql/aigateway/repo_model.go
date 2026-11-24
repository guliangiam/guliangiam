package aigateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ============================================================================
// ModelsRepo
// ============================================================================

type ModelsRepository struct {
	db mysqlshared.DBTX
}

func NewModelsRepository(db mysqlshared.DBTX) ModelsRepository {
	return ModelsRepository{db: db}
}

var _ domain.ModelsRepo = (*ModelsRepository)(nil)
var _ domain.TenantScopedModelsRepo = (*ModelsRepository)(nil)

func (r *ModelsRepository) GetByID(ctx context.Context, id string) (*domain.LlmModel, error) {
	row := r.db.QueryRowContext(ctx, queryModelGetByID, id)
	m, err := scanLlmModel(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("model not found")
		}
		return nil, err
	}
	return m, nil
}

// GetByIDForOrganization is the management by-ID read. The tenant predicate is
// explicit because MySQL has no RLS, so a foreign or missing id is NotFound.
func (r *ModelsRepository) GetByIDForOrganization(ctx context.Context, organizationID, id string) (*domain.LlmModel, error) {
	row := r.db.QueryRowContext(ctx, queryModelGetByIDForOrganization, id, organizationID)
	m, err := scanLlmModel(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("model not found")
		}
		return nil, err
	}
	return m, nil
}

func (r *ModelsRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmModel, error) {
	rows, err := r.db.QueryContext(ctx, queryModelList, organizationID)
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

func (r *ModelsRepository) ListByProvider(ctx context.Context, providerID string) ([]*domain.LlmModel, error) {
	rows, err := r.db.QueryContext(ctx, queryModelListByProvider, providerID)
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

func (r *ModelsRepository) Create(ctx context.Context, m *domain.LlmModel) error {
	caps, err := json.Marshal(m.Capabilities)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryModelInsert,
		m.ID, m.OrganizationID, m.ProviderID, m.ProviderModelName, m.DisplayName,
		string(caps), m.MaxContextTokens, m.MaxOutputTokens,
		m.Status, m.CreatedAt, m.UpdatedAt, m.DeletedAt, m.Revision,
	)
	return err
}

func (r *ModelsRepository) Update(ctx context.Context, m *domain.LlmModel) error {
	caps, err := json.Marshal(m.Capabilities)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, queryModelUpdate,
		m.ProviderModelName, m.DisplayName, string(caps),
		m.MaxContextTokens, m.MaxOutputTokens, m.Status,
		m.UpdatedAt, m.DeletedAt,
		m.ID, m.Revision,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewConflictf("model revision conflict (id=%s)", m.ID)
	}
	m.Revision++
	return nil
}

func (r *ModelsRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, queryModelDelete, id)
	return err
}

// UpdateForOrganization persists the aggregate only inside organizationID.
// The explicit tenant predicate plus revision guard means a foreign id is a
// conflict, never a cross-tenant update.
func (r *ModelsRepository) UpdateForOrganization(ctx context.Context, m *domain.LlmModel, organizationID string) error {
	caps, err := json.Marshal(m.Capabilities)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, queryModelUpdateForOrganization,
		m.ProviderModelName, m.DisplayName, string(caps),
		m.MaxContextTokens, m.MaxOutputTokens, m.Status,
		m.UpdatedAt, m.DeletedAt,
		m.ID, m.Revision, organizationID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewConflictf("model revision conflict (id=%s)", m.ID)
	}
	m.Revision++
	return nil
}

// DeleteForOrganization soft-deletes only a row owned by organizationID. A
// foreign or already deleted id affects no rows and is reported as NotFound.
func (r *ModelsRepository) DeleteForOrganization(ctx context.Context, organizationID, id string) error {
	res, err := r.db.ExecContext(ctx, queryModelDeleteForOrganization, id, organizationID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewNotFound("model not found")
	}
	return nil
}

// ============================================================================
// AliasesRepo
// ============================================================================

type AliasesRepository struct {
	db mysqlshared.DBTX
}

func NewAliasesRepository(db mysqlshared.DBTX) AliasesRepository {
	return AliasesRepository{db: db}
}

var _ domain.AliasesRepo = (*AliasesRepository)(nil)
var _ domain.TenantScopedAliasesRepo = (*AliasesRepository)(nil)

func (r *AliasesRepository) GetByID(ctx context.Context, id string) (*domain.LlmModelAlias, error) {
	row := r.db.QueryRowContext(ctx, queryAliasGetByID, id)
	a, err := scanLlmModelAlias(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("alias not found")
		}
		return nil, err
	}
	return a, nil
}

// GetByIDForOrganization is the management by-ID read. The tenant predicate is
// explicit because MySQL has no RLS, so a foreign or missing id is NotFound.
func (r *AliasesRepository) GetByIDForOrganization(ctx context.Context, organizationID, id string) (*domain.LlmModelAlias, error) {
	row := r.db.QueryRowContext(ctx, queryAliasGetByIDForOrganization, id, organizationID)
	a, err := scanLlmModelAlias(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("alias not found")
		}
		return nil, err
	}
	return a, nil
}

func (r *AliasesRepository) GetByName(ctx context.Context, organizationID, alias string) (*domain.LlmModelAlias, error) {
	row := r.db.QueryRowContext(ctx, queryAliasGetByName, organizationID, alias)
	a, err := scanLlmModelAlias(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("alias not found")
		}
		return nil, err
	}
	return a, nil
}

func (r *AliasesRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmModelAlias, error) {
	rows, err := r.db.QueryContext(ctx, queryAliasList, organizationID)
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

func (r *AliasesRepository) ListAll(ctx context.Context) ([]*domain.LlmModelAlias, error) {
	rows, err := r.db.QueryContext(ctx, queryAliasListAll)
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

func (r *AliasesRepository) Create(ctx context.Context, a *domain.LlmModelAlias) error {
	_, err := r.db.ExecContext(ctx, queryAliasInsert,
		a.ID, a.OrganizationID, a.Alias, a.DisplayName,
		a.Status, a.CreatedAt, a.UpdatedAt, a.DeletedAt, a.Revision,
	)
	return err
}

func (r *AliasesRepository) Update(ctx context.Context, a *domain.LlmModelAlias) error {
	res, err := r.db.ExecContext(ctx, queryAliasUpdate,
		a.Alias, a.DisplayName, a.Status,
		a.UpdatedAt, a.DeletedAt,
		a.ID, a.Revision,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewConflictf("alias revision conflict (id=%s)", a.ID)
	}
	a.Revision++
	return nil
}

func (r *AliasesRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, queryAliasDelete, id)
	return err
}

// UpdateForOrganization persists the aggregate only inside organizationID.
// The explicit tenant predicate plus revision guard means a foreign id is a
// conflict, never a cross-tenant update.
func (r *AliasesRepository) UpdateForOrganization(ctx context.Context, a *domain.LlmModelAlias, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryAliasUpdateForOrganization,
		a.Alias, a.DisplayName, a.Status,
		a.UpdatedAt, a.DeletedAt,
		a.ID, a.Revision, organizationID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewConflictf("alias revision conflict (id=%s)", a.ID)
	}
	a.Revision++
	return nil
}

// DeleteForOrganization soft-deletes only a row owned by organizationID. A
// foreign or already deleted id affects no rows and is reported as NotFound.
func (r *AliasesRepository) DeleteForOrganization(ctx context.Context, organizationID, id string) error {
	res, err := r.db.ExecContext(ctx, queryAliasDeleteForOrganization, id, organizationID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewNotFound("alias not found")
	}
	return nil
}

// ============================================================================
// PriceVersionsRepo
// ============================================================================

type PriceVersionsRepository struct {
	db mysqlshared.DBTX
}

func NewPriceVersionsRepository(db mysqlshared.DBTX) PriceVersionsRepository {
	return PriceVersionsRepository{db: db}
}

var _ domain.PriceVersionsRepo = (*PriceVersionsRepository)(nil)
var _ domain.TenantScopedPriceVersionsRepo = (*PriceVersionsRepository)(nil)

func (r *PriceVersionsRepository) GetByID(ctx context.Context, id string) (*domain.LlmPriceVersion, error) {
	row := r.db.QueryRowContext(ctx, queryPriceVersionGetByID, id)
	v, err := scanLlmPriceVersion(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("price version not found")
		}
		return nil, err
	}
	return v, nil
}

// GetByIDForOrganization is the management by-ID read. The tenant predicate is
// explicit because MySQL has no RLS, so a foreign or missing id is NotFound.
func (r *PriceVersionsRepository) GetByIDForOrganization(ctx context.Context, organizationID, id string) (*domain.LlmPriceVersion, error) {
	row := r.db.QueryRowContext(ctx, queryPriceVersionGetByIDForOrganization, id, organizationID)
	v, err := scanLlmPriceVersion(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("price version not found")
		}
		return nil, err
	}
	return v, nil
}

func (r *PriceVersionsRepository) GetByModel(ctx context.Context, modelID string) (*domain.LlmPriceVersion, error) {
	row := r.db.QueryRowContext(ctx, queryPriceVersionGetByModel, modelID)
	v, err := scanLlmPriceVersion(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("price version not found")
		}
		return nil, err
	}
	return v, nil
}

func (r *PriceVersionsRepository) ListByModel(ctx context.Context, modelID string) ([]*domain.LlmPriceVersion, error) {
	rows, err := r.db.QueryContext(ctx, queryPriceVersionListByModel, modelID)
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

func (r *PriceVersionsRepository) Create(ctx context.Context, v *domain.LlmPriceVersion) error {
	_, err := r.db.ExecContext(ctx, queryPriceVersionInsert,
		v.ID, v.OrganizationID, v.ModelID, v.VersionNo,
		v.InputPricePer1M, v.OutputPricePer1M, v.Currency, v.EffectiveFrom,
		v.CreatedAt, v.UpdatedAt, v.DeletedAt, v.Revision,
	)
	return err
}

func (r *PriceVersionsRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmPriceVersion, error) {
	rows, err := r.db.QueryContext(ctx, queryPriceVersionList, organizationID)
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

func (r *PriceVersionsRepository) Update(ctx context.Context, v *domain.LlmPriceVersion) error {
	res, err := r.db.ExecContext(ctx, queryPriceVersionUpdate,
		v.VersionNo, v.InputPricePer1M, v.OutputPricePer1M,
		v.Currency, v.EffectiveFrom, v.UpdatedAt, v.ID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewNotFound("price version not found")
	}
	return nil
}

func (r *PriceVersionsRepository) Delete(ctx context.Context, id string) error {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, queryPriceVersionSoftDelete, now, now, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewNotFound("price version not found")
	}
	return nil
}

// UpdateForOrganization persists the aggregate only inside organizationID.
// The explicit tenant predicate plus current-revision guard means a foreign id
// is a conflict, never a cross-tenant update.
func (r *PriceVersionsRepository) UpdateForOrganization(ctx context.Context, v *domain.LlmPriceVersion, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryPriceVersionUpdateForOrganization,
		v.VersionNo, v.InputPricePer1M, v.OutputPricePer1M,
		v.Currency, v.EffectiveFrom, v.UpdatedAt,
		v.ID, v.Revision, organizationID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewConflictf("price version revision conflict (id=%s)", v.ID)
	}
	v.Revision++
	return nil
}

// DeleteForOrganization soft-deletes only a row owned by organizationID. A
// foreign or already deleted id affects no rows and is reported as NotFound.
func (r *PriceVersionsRepository) DeleteForOrganization(ctx context.Context, organizationID, id string) error {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, queryPriceVersionDeleteForOrganization, now, now, id, organizationID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewNotFound("price version not found")
	}
	return nil
}
