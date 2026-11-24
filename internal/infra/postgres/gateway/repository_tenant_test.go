package gateway

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_GetSiteByIDForOrganization_UsesTenantPredicate(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow([]any{
				"site-1", "org-1", "portal", "portal.example.com", "active",
				"auto", nil, true,
				true, 30, "healthy", nil,
				false, 0, 0, []string{},
				[]byte(`{}`), nil, nil, nil, int64(1),
			}, nil)
		},
	}

	repo := NewRepository(mockDB)
	site, err := repo.GetSiteByIDForOrganization(context.Background(), "site-1", "org-1")
	require.NoError(t, err)
	require.NotNil(t, site)
	assert.Equal(t, "org-1", site.OrganizationID)
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"site-1", "org-1"}, gotArgs)
}

func TestRepository_GetSiteByIDForOrganization_NotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(context.Context, string, ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}

	repo := NewRepository(mockDB)
	site, err := repo.GetSiteByIDForOrganization(context.Background(), "foreign-site", "org-1")
	require.Error(t, err)
	require.Nil(t, site)
	assert.True(t, strings.Contains(err.Error(), "site not found"))
}

func TestRepository_GetSiteByIDForOrganization_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(context.Context, string, ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}

	repo := NewRepository(mockDB)
	_, err := repo.GetSiteByIDForOrganization(context.Background(), "site-1", "org-1")
	require.Error(t, err)
}

func TestSiteByIDForOrganizationQueryKeepsSystemLookup(t *testing.T) {
	assert.Contains(t, queryGetSiteByID, "WHERE id = $1::uuid AND deleted_at IS NULL")
	assert.NotContains(t, queryGetSiteByID, "WHERE id = $1::uuid AND organization_id")
	assert.Contains(t, queryGetSiteByIDForOrganization, "organization_id = guliangiam.resolve_org_id($2)")
}

func TestRepository_GetServerByIDForOrganization_UsesTenantPredicate(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow([]any{
				"server-1", "org-1", "gateway", "https://gw.example.com", "https", "active",
				[]byte(`{}`), nil, nil, nil, int64(1),
			}, nil)
		},
	}

	repo := NewRepository(mockDB)
	server, err := repo.GetServerByIDForOrganization(context.Background(), "server-1", "org-1")
	require.NoError(t, err)
	require.NotNil(t, server)
	assert.Equal(t, "org-1", server.OrganizationID)
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"server-1", "org-1"}, gotArgs)
}

func TestRepository_GetServerByIDForOrganization_NotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(context.Context, string, ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}

	repo := NewRepository(mockDB)
	server, err := repo.GetServerByIDForOrganization(context.Background(), "foreign-server", "org-1")
	require.Error(t, err)
	require.Nil(t, server)
	assert.True(t, strings.Contains(err.Error(), "server not found"))
}

func TestServerByIDForOrganizationQueryKeepsSystemLookup(t *testing.T) {
	assert.Contains(t, queryGetServerByID, "WHERE id = $1::uuid AND deleted_at IS NULL")
	assert.NotContains(t, queryGetServerByID, "WHERE id = $1::uuid AND organization_id")
	assert.Contains(t, queryGetServerByIDForOrganization, "organization_id = guliangiam.resolve_org_id($2)")
}

var _ domain.Repository = Repository{}
