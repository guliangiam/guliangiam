package scim

import (

	// stdlib errors retained for errors.Is/errors.As only; apperrors used for error creation

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSchemaNotFound = apperrors.NewNotFound("scim schema not found")
	ErrUserNotFound   = apperrors.NewNotFound("scim user not found")
	ErrGroupNotFound  = apperrors.NewNotFound("scim group not found")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ==================== ScimSchema Operations ====================
