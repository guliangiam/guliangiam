package aigateway

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// ============================================================================
// RoutesRepo
// ============================================================================

type RoutesRepository struct {
	db shared.DBTX
}

func NewRoutesRepository(db shared.DBTX) RoutesRepository {
	return RoutesRepository{db: db}
}

var _ domain.RoutesRepo = RoutesRepository{}

func (r RoutesRepository) GetByID(ctx context.Context, id string) (*domain.LlmRoute, error) {
	row := r.db.QueryRow(ctx, queryRouteGetByID, id)
	rt, err := scanLlmRoute(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("route not found")
		}
		return nil, err
	}
	return rt, nil
}

func (r RoutesRepository) ListByAlias(ctx context.Context, aliasID string) ([]*domain.LlmRoute, error) {
	rows, err := r.db.Query(ctx, queryRouteListByAlias, aliasID)
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

func (r RoutesRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmRoute, error) {
	rows, err := r.db.Query(ctx, queryRouteList, organizationID)
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

func (r RoutesRepository) Create(ctx context.Context, rt *domain.LlmRoute) error {
	_, err := r.db.Exec(ctx, queryRouteInsert,
		rt.ID, rt.OrganizationID, rt.AliasID, rt.ModelID,
		rt.Weight, rt.Priority, rt.Status,
		rt.CreatedAt, rt.UpdatedAt, rt.DeletedAt, rt.Revision,
	)
	return err
}

func (r RoutesRepository) Update(ctx context.Context, rt *domain.LlmRoute) error {
	var newRevision int64
	err := r.db.QueryRow(ctx, queryRouteUpdate,
		rt.ModelID, rt.Weight, rt.Priority, rt.Status,
		rt.UpdatedAt, rt.DeletedAt,
		rt.Revision-1, rt.ID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("route revision conflict")
		}
		return err
	}
	rt.Revision = newRevision
	return nil
}

func (r RoutesRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, queryRouteDelete, id)
	return err
}

// ============================================================================
// ApiKeysRepo
// ============================================================================

type ApiKeysRepository struct {
	db shared.DBTX
}

func NewApiKeysRepository(db shared.DBTX) ApiKeysRepository {
	return ApiKeysRepository{db: db}
}

var _ domain.ApiKeysRepo = ApiKeysRepository{}

func (r ApiKeysRepository) GetByID(ctx context.Context, id string) (*domain.LlmApiKey, error) {
	row := r.db.QueryRow(ctx, queryAPIKeyGetByID, id)
	k, err := scanLlmApiKey(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("api key not found")
		}
		return nil, err
	}
	return k, nil
}

func (r ApiKeysRepository) GetByHash(ctx context.Context, hash string) (*domain.LlmApiKey, error) {
	row := r.db.QueryRow(ctx, queryAPIKeyGetByHash, hash)
	k, err := scanLlmApiKey(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("api key not found")
		}
		return nil, err
	}
	return k, nil
}

func (r ApiKeysRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmApiKey, error) {
	rows, err := r.db.Query(ctx, queryAPIKeyList, organizationID)
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

func (r ApiKeysRepository) Create(ctx context.Context, k *domain.LlmApiKey) error {
	aliases, err := shared.MarshalJSON(k.AllowedAliases)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryAPIKeyInsert,
		k.ID, k.OrganizationID, k.Name, k.KeyPrefix, k.KeyHash,
		aliases, k.RPMLimit, k.TPMLimit, k.MonthlyBudget,
		k.Currency, k.Status, k.ExpiresAt,
		k.CreatedAt, k.UpdatedAt, k.DeletedAt, k.Revision,
	)
	return err
}

func (r ApiKeysRepository) Update(ctx context.Context, k *domain.LlmApiKey) error {
	aliases, err := shared.MarshalJSON(k.AllowedAliases)
	if err != nil {
		return err
	}
	var newRevision int64
	err = r.db.QueryRow(ctx, queryAPIKeyUpdate,
		k.Name, k.KeyPrefix, aliases,
		k.RPMLimit, k.TPMLimit, k.MonthlyBudget,
		k.Currency, k.Status, k.ExpiresAt,
		k.UpdatedAt, k.DeletedAt,
		k.Revision-1, k.ID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("api key revision conflict")
		}
		return err
	}
	k.Revision = newRevision
	return nil
}

func (r ApiKeysRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, queryAPIKeyDelete, id)
	return err
}
