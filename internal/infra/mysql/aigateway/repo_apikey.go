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
// RoutesRepo
// ============================================================================

type RoutesRepository struct {
	db mysqlshared.DBTX
}

func NewRoutesRepository(db mysqlshared.DBTX) RoutesRepository {
	return RoutesRepository{db: db}
}

var _ domain.RoutesRepo = (*RoutesRepository)(nil)

func (r *RoutesRepository) GetByID(ctx context.Context, id string) (*domain.LlmRoute, error) {
	row := r.db.QueryRowContext(ctx, queryRouteGetByID, id)
	rt, err := scanLlmRoute(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("route not found")
		}
		return nil, err
	}
	return rt, nil
}

func (r *RoutesRepository) ListByAlias(ctx context.Context, aliasID string) ([]*domain.LlmRoute, error) {
	rows, err := r.db.QueryContext(ctx, queryRouteListByAlias, aliasID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := make([]*domain.LlmRoute, 0, 8)
	for rows.Next() {
		rt, err := scanLlmRoute(rows)
		if err != nil {
			return nil, err
		}
		routes = append(routes, rt)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return routes, nil
}

func (r *RoutesRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmRoute, error) {
	rows, err := r.db.QueryContext(ctx, queryRouteList, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := make([]*domain.LlmRoute, 0, 16)
	for rows.Next() {
		rt, err := scanLlmRoute(rows)
		if err != nil {
			return nil, err
		}
		routes = append(routes, rt)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return routes, nil
}

func (r *RoutesRepository) Create(ctx context.Context, rt *domain.LlmRoute) error {
	_, err := r.db.ExecContext(ctx, queryRouteInsert,
		rt.ID, rt.OrganizationID, rt.AliasID, rt.ModelID,
		rt.Weight, rt.Priority, rt.Status,
		rt.CreatedAt, rt.UpdatedAt, rt.DeletedAt, rt.Revision,
	)
	return err
}

func (r *RoutesRepository) Update(ctx context.Context, rt *domain.LlmRoute) error {
	res, err := r.db.ExecContext(ctx, queryRouteUpdate,
		rt.ModelID, rt.Weight, rt.Priority, rt.Status,
		rt.UpdatedAt, rt.DeletedAt,
		rt.ID, rt.Revision,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewConflictf("route revision conflict (id=%s)", rt.ID)
	}
	rt.Revision++
	return nil
}

func (r *RoutesRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, queryRouteDelete, id)
	return err
}

// ============================================================================
// ApiKeysRepo
// ============================================================================

type ApiKeysRepository struct {
	db mysqlshared.DBTX
}

func NewApiKeysRepository(db mysqlshared.DBTX) ApiKeysRepository {
	return ApiKeysRepository{db: db}
}

var _ domain.ApiKeysRepo = (*ApiKeysRepository)(nil)

func (r *ApiKeysRepository) GetByID(ctx context.Context, id string) (*domain.LlmApiKey, error) {
	row := r.db.QueryRowContext(ctx, queryAPIKeyGetByID, id)
	k, err := scanLlmApiKey(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("api key not found")
		}
		return nil, err
	}
	return k, nil
}

func (r *ApiKeysRepository) GetByHash(ctx context.Context, hash string) (*domain.LlmApiKey, error) {
	row := r.db.QueryRowContext(ctx, queryAPIKeyGetByHash, hash)
	k, err := scanLlmApiKey(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("api key not found")
		}
		return nil, err
	}
	return k, nil
}

func (r *ApiKeysRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmApiKey, error) {
	rows, err := r.db.QueryContext(ctx, queryAPIKeyList, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]*domain.LlmApiKey, 0, 16)
	for rows.Next() {
		k, err := scanLlmApiKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return keys, nil
}

func (r *ApiKeysRepository) Create(ctx context.Context, k *domain.LlmApiKey) error {
	aliases, err := json.Marshal(k.AllowedAliases)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryAPIKeyInsert,
		k.ID, k.OrganizationID, k.Name, k.KeyPrefix, k.KeyHash,
		string(aliases), k.RPMLimit, k.TPMLimit, k.MonthlyBudget,
		k.Currency, k.Status, k.ExpiresAt,
		k.CreatedAt, k.UpdatedAt, k.DeletedAt, k.Revision,
	)
	return err
}

func (r *ApiKeysRepository) Update(ctx context.Context, k *domain.LlmApiKey) error {
	aliases, err := json.Marshal(k.AllowedAliases)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, queryAPIKeyUpdate,
		k.Name, k.KeyPrefix, string(aliases),
		k.RPMLimit, k.TPMLimit, k.MonthlyBudget,
		k.Currency, k.Status, k.ExpiresAt,
		k.UpdatedAt, k.DeletedAt,
		k.ID, k.Revision,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewConflictf("api key revision conflict (id=%s)", k.ID)
	}
	k.Revision++
	return nil
}

func (r *ApiKeysRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, queryAPIKeyDelete, id)
	return err
}
