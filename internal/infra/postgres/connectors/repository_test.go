package connectors

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	domainconnectors "github.com/guliangiam/guliangiam/internal/domain/connectors"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func testEncryptor(t *testing.T) *crypto.FieldEncryptor {
	t.Helper()
	enc, err := crypto.NewFieldEncryptorChecked()
	require.NoError(t, err)
	return enc
}

func requireErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, code, appErr.Code)
}

func TestRepository_CreateSealsBearerToken(t *testing.T) {
	enc := testEncryptor(t)
	const plaintext = "bearer-plaintext-not-for-db"

	var gotArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
			gotArgs = append([]any(nil), args...)
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(db, enc)
	connector := &domainconnectors.Connector{
		ID:            "connector-1",
		Name:          "scim",
		SCIMBaseURL:   "https://scim.example.com/v2",
		BearerToken:   plaintext,
		IsEnabled:     true,
		TimeoutMs:     domainconnectors.DefaultTimeoutMs,
		MaxRatePerMin: domainconnectors.DefaultMaxRatePerMin,
	}

	require.NoError(t, repo.Create(context.Background(), "org-1", connector))
	require.Len(t, gotArgs, 11)
	sealed, ok := gotArgs[4].(string)
	require.True(t, ok)
	require.True(t, crypto.IsEnvelopeToken(sealed), "persisted token must be an envelope token")
	require.NotEqual(t, plaintext, sealed)
	require.Equal(t, "org-1", connector.OrganizationID)
	require.Equal(t, int64(1), connector.Revision)
	require.False(t, connector.CreatedAt.IsZero())
	require.False(t, connector.UpdatedAt.IsZero())
}

func TestRepository_CreateFailsClosedWithoutEncryptor(t *testing.T) {
	execCalled := false
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			execCalled = true
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(db, nil)
	err := repo.Create(context.Background(), "org-1", &domainconnectors.Connector{
		ID:          "connector-1",
		BearerToken: "must-not-be-stored-in-plaintext",
	})
	requireErrorCode(t, err, apperrors.CodeInternal)
	require.False(t, execCalled, "encryption failure must prevent the INSERT")
	require.NotContains(t, err.Error(), "must-not-be-stored-in-plaintext")
}

func TestRepository_GetDecryptsBearerToken(t *testing.T) {
	enc := testEncryptor(t)
	const plaintext = "bearer-plaintext-in-memory"
	sealed, err := enc.EncryptColumn(bearerTokenAAD("connector-1"), plaintext)
	require.NoError(t, err)

	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			require.Contains(t, sql, "organization_id = $2::uuid")
			return shared.NewMockRow([]any{
				"connector-1", "org-1", "scim", "https://scim.example.com/v2",
				sealed, true, domainconnectors.DefaultTimeoutMs,
				domainconnectors.DefaultMaxRatePerMin, int64(1), nil, nil,
			}, nil)
		},
	}
	repo := NewRepository(db, enc)
	got, err := repo.Get(context.Background(), "connector-1", "org-1")
	require.NoError(t, err)
	require.Equal(t, plaintext, got.BearerToken)
	require.Equal(t, "org-1", got.OrganizationID)
	require.Equal(t, int64(1), got.Revision)
}

func TestRepository_GetNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	_, err := NewRepository(db, testEncryptor(t)).Get(context.Background(), "connector-1", "org-1")
	requireErrorCode(t, err, apperrors.CodeNotFound)
}

func TestRepository_GetRejectsPlaintextAtRest(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return shared.NewMockRow([]any{
				"connector-1", "org-1", "scim", "https://scim.example.com/v2",
				"plaintext-at-rest", true, 5000, 300, int64(1), nil, nil,
			}, nil)
		},
	}
	_, err := NewRepository(db, testEncryptor(t)).Get(context.Background(), "connector-1", "org-1")
	requireErrorCode(t, err, apperrors.CodeInternal)
	require.NotContains(t, err.Error(), "plaintext-at-rest")
}

func TestRepository_UpdateSuccessBumpsRevision(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = append([]any(nil), args...)
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(db, testEncryptor(t))
	connector := &domainconnectors.Connector{
		ID:             "connector-1",
		OrganizationID: "org-1",
		Name:           "scim-updated",
		SCIMBaseURL:    "https://scim.example.com/v2",
		BearerToken:    "rotated-token",
		IsEnabled:      false,
		TimeoutMs:      8000,
		MaxRatePerMin:  120,
		Revision:       1,
	}

	require.NoError(t, repo.Update(context.Background(), "org-1", connector))
	require.Equal(t, int64(2), connector.Revision)
	require.Contains(t, gotSQL, "organization_id = $9::uuid")
	require.Contains(t, gotSQL, "revision = revision + 1")
	require.Contains(t, gotSQL, "AND revision = $10")
	require.Len(t, gotArgs, 10)
	require.Equal(t, "connector-1", gotArgs[7])
	require.Equal(t, "org-1", gotArgs[8])
	require.Equal(t, int64(1), gotArgs[9], "the guard uses the revision read by the caller")
}

func TestRepository_UpdateRevisionConflict(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db, testEncryptor(t))
	err := repo.Update(context.Background(), "org-1", &domainconnectors.Connector{
		ID:       "connector-1",
		Name:     "stale",
		Revision: 1,
	})
	requireErrorCode(t, err, apperrors.CodeConflict)
}

func TestRepository_DeleteNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
			require.Contains(t, sql, "organization_id = $2::uuid")
			return pgconn.NewCommandTag("DELETE 0"), nil
		},
	}
	err := NewRepository(db, testEncryptor(t)).Delete(context.Background(), "org-1", "connector-1")
	requireErrorCode(t, err, apperrors.CodeNotFound)
}

func TestRepository_ListScopesAndDecrypts(t *testing.T) {
	enc := testEncryptor(t)
	sealed, err := enc.EncryptColumn(bearerTokenAAD("connector-1"), "list-token")
	require.NoError(t, err)

	db := &shared.MockDBTX{
		QueryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			require.Contains(t, sql, "WHERE organization_id = $1::uuid")
			return shared.NewMockRows([][]any{
				{"connector-1", "org-1", "a", "https://a.example.com", sealed, true, 5000, 300, int64(1), nil, nil},
			}), nil
		},
	}
	items, err := NewRepository(db, enc).List(context.Background(), "org-1")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "connector-1", items[0].ID)
	require.Equal(t, "org-1", items[0].OrganizationID)
	require.Equal(t, "list-token", items[0].BearerToken)
}

func TestRepository_ErrorsDoNotEchoSecrets(t *testing.T) {
	enc := testEncryptor(t)
	const plaintext = "super-secret-bearer"
	sealed, err := enc.EncryptColumn(bearerTokenAAD("connector-1"), plaintext)
	require.NoError(t, err)
	require.NotEqual(t, plaintext, sealed)

	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db failure while reading row"))
		},
	}
	_, err = NewRepository(db, enc).Get(context.Background(), "connector-1", "org-1")
	require.Error(t, err)
	require.NotContains(t, err.Error(), plaintext)
	require.NotContains(t, err.Error(), sealed)
	require.False(t, strings.Contains(err.Error(), "v1-"), "ciphertext prefix must not leak")
}
