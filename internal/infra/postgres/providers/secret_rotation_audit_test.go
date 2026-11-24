package providers

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestNewSecretRotationAuditRepository(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewSecretRotationAuditRepository(db)
	assert.NotNil(t, repo)
}

func TestSecretRotationAuditRepository_Create_Success(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT"), nil
		},
	}
	repo := NewSecretRotationAuditRepository(db)
	err := repo.Create(context.Background(), &domain.SecretRotationAudit{
		ID:         "audit1",
		ProviderID: "prov1",
		SecretType: "client_secret",
	})
	assert.NoError(t, err)
}

func TestSecretRotationAuditRepository_Create_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewSecretRotationAuditRepository(db)
	err := repo.Create(context.Background(), &domain.SecretRotationAudit{ID: "audit1"})
	assert.Error(t, err)
}

func TestSecretRotationAuditRepository_ListByProviderID_QueryError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := NewSecretRotationAuditRepository(db)
	_, err := repo.ListByProviderID(context.Background(), "prov1", 10)
	assert.Error(t, err)
}

func TestSecretRotationAuditRepository_ListByProviderID_Success(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return shared.NewMockRows([][]any{}), nil
		},
	}
	repo := NewSecretRotationAuditRepository(db)
	items, err := repo.ListByProviderID(context.Background(), "prov1", 10)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestSecretRotationAuditRepository_ListByProviderID_DefaultLimit(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return shared.NewMockRows([][]any{}), nil
		},
	}
	repo := NewSecretRotationAuditRepository(db)
	items, err := repo.ListByProviderID(context.Background(), "prov1", 0)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestRecordSecretRotation_Success(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT"), nil
		},
	}
	repo := NewSecretRotationAuditRepository(db)
	err := RecordSecretRotation(context.Background(), repo, "prov1", "client_secret", 1, 2, "admin", "scheduled rotation")
	assert.NoError(t, err)
}

func TestRecordSecretRotation_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := NewSecretRotationAuditRepository(db)
	err := RecordSecretRotation(context.Background(), repo, "prov1", "client_secret", 1, 2, "admin", "scheduled rotation")
	assert.Error(t, err)
}
