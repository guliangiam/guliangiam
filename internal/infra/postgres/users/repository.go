package users

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
)

// beginner is implemented by both pgx.Pool and pgx.Tx (which uses savepoints).
// This allows withinTransaction to work whether r.db is a pool connection or
// an already-active transaction.
type beginner interface {
	shared.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

// withinTransaction executes fn inside a database transaction. If r.db implements
// beginner (i.e. is a pgx.Pool or pgx.Tx), a new transaction or savepoint is
// opened. Otherwise fn is called directly (backward compatibility for test mocks
// that only implement DBTX).
func (r Repository) withinTransaction(ctx context.Context, fn func(ctx context.Context, tx shared.DBTX) error) error {
	if b, ok := r.db.(beginner); ok {
		tx, err := b.Begin(ctx)
		if err != nil {
			return err
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback(ctx)
			}
		}()

		if err := fn(ctx, tx); err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return err
		}
		committed = true
		return nil
	}

	// r.db does not support Begin (e.g. test mock); execute directly.
	return fn(ctx, r.db)
}

var _ domain.Repository = Repository{}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

const queryBatchLoadProfiles = `
SELECT user_id, COALESCE(display_name, ''), COALESCE(first_name, ''), COALESCE(last_name, ''), COALESCE(avatar_url, ''), COALESCE(email, ''), email_verified, COALESCE(phone, ''), phone_verified, COALESCE(country_code, ''), COALESCE(region, ''), COALESCE(location, ''), COALESCE(birthday::text, ''), COALESCE(gender, ''), COALESCE(bio, ''), COALESCE(real_name, ''), COALESCE(id_card_type, ''), COALESCE(id_card, ''), COALESCE(passport_no, ''), COALESCE(driver_license_no, ''), id_card_sealed, passport_no_sealed, driver_license_no_sealed, COALESCE(id_card_expiry::text, ''), COALESCE(issue_country, ''), COALESCE(issuing_authority, ''), COALESCE(passport_photo_url, ''), COALESCE(driver_license_front_url, ''), COALESCE(driver_license_back_url, ''), properties
FROM guliangiam.user_profiles
WHERE user_id = ANY($1::uuid[])`

// batchLoadProfiles loads profiles for a batch of users in a single query.
