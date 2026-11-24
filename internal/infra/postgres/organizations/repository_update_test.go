package organizations

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestUpdate_QueryRowError(t *testing.T) {
	ctx := context.Background()
	organization := &domain.Organization{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, organization)
	assert.Error(t, err)
}

func TestUpdate_RevisionConflict(t *testing.T) {
	ctx := context.Background()
	organization := &domain.Organization{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, organization)
	assert.Error(t, err)
}

func TestUpdate_Success(t *testing.T) {
	ctx := context.Background()
	organization := &domain.Organization{}
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(ctx, organization)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), organization.Revision)
}

// TestUpdate_NilStringSlicesBecomeEmptyArrays covers the update side of the
// shared upsert: the full-replace statement must bind empty arrays instead of
// SQL NULL for password_options / ldap_attributes (NOT NULL varchar[] columns).
func TestUpdate_NilStringSlicesBecomeEmptyArrays(t *testing.T) {
	ctx := context.Background()
	var authArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(context.Context, string, ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "organization_auth_config") {
				authArgs = append([]any(nil), args...)
			}
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)

	err := repo.Update(ctx, &domain.Organization{
		ID:              "01900000-0000-7000-8000-000000000001",
		PasswordOptions: nil,
		LdapAttributes:  nil,
		Revision:        1,
	})
	require.NoError(t, err)

	require.Len(t, authArgs, 11, "organization_auth_config upsert argument count")
	require.Equal(t, []string{}, authArgs[2], "password_options must never bind as NULL")
	require.Equal(t, []string{}, authArgs[10], "ldap_attributes must never bind as NULL")
}

func TestUpdateSettings_ExecError(t *testing.T) {
	ctx := context.Background()
	organizationSettings := &domain.OrganizationSettings{}
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateSettings(ctx, organizationSettings)
	assert.Error(t, err)
}

func TestUpdateSettings_Success(t *testing.T) {
	ctx := context.Background()
	organizationSettings := &domain.OrganizationSettings{}
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateSettings(ctx, organizationSettings)
	assert.NoError(t, err)
}

func TestUpdateTheme_ExecError(t *testing.T) {
	ctx := context.Background()
	organizationThemeSettings := &domain.OrganizationThemeSettings{}
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateTheme(ctx, organizationThemeSettings)
	assert.Error(t, err)
}

func TestUpdateTheme_Success(t *testing.T) {
	ctx := context.Background()
	organizationThemeSettings := &domain.OrganizationThemeSettings{}
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateTheme(ctx, organizationThemeSettings)
	assert.NoError(t, err)
}

func TestUpdateSecurityPolicy_ExecError(t *testing.T) {
	ctx := context.Background()
	organizationSecurityPolicy := &domain.OrganizationSecurityPolicy{}
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateSecurityPolicy(ctx, organizationSecurityPolicy)
	assert.Error(t, err)
}

func TestUpdateSecurityPolicy_Success(t *testing.T) {
	ctx := context.Background()
	organizationSecurityPolicy := &domain.OrganizationSecurityPolicy{}
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.UpdateSecurityPolicy(ctx, organizationSecurityPolicy)
	assert.NoError(t, err)
}
