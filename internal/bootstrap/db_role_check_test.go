package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerifyPostgresRoleIsolation(t *testing.T) {
	t.Run("non_superuser_without_bypass_passes", func(t *testing.T) {
		query := func(context.Context) (postgresRoleAttributes, error) {
			return postgresRoleAttributes{Name: "guliangiam_app", Super: false, BypassRLS: false}, nil
		}
		require.NoError(t, verifyPostgresRoleIsolation(context.Background(), query))
	})

	t.Run("superuser_fails_with_role_name", func(t *testing.T) {
		query := func(context.Context) (postgresRoleAttributes, error) {
			return postgresRoleAttributes{Name: "postgres", Super: true, BypassRLS: false}, nil
		}
		err := verifyPostgresRoleIsolation(context.Background(), query)
		require.Error(t, err)
		require.Contains(t, err.Error(), `"postgres"`)
		require.Contains(t, err.Error(), "rolsuper=true")
		require.Contains(t, err.Error(), "rolbypassrls=false")
	})

	t.Run("bypassrls_fails", func(t *testing.T) {
		query := func(context.Context) (postgresRoleAttributes, error) {
			return postgresRoleAttributes{Name: "guliangiam_rw", Super: false, BypassRLS: true}, nil
		}
		err := verifyPostgresRoleIsolation(context.Background(), query)
		require.Error(t, err)
		require.Contains(t, err.Error(), `"guliangiam_rw"`)
		require.Contains(t, err.Error(), "rolbypassrls", "should name the bypass attribute")
	})

	t.Run("query_error_fails_closed", func(t *testing.T) {
		query := func(context.Context) (postgresRoleAttributes, error) {
			return postgresRoleAttributes{}, errors.New("pg_roles unavailable")
		}
		err := verifyPostgresRoleIsolation(context.Background(), query)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot verify PostgreSQL role isolation attributes")
	})
}
