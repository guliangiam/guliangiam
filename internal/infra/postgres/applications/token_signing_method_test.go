package applications

import (
	"context"
	"strings"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// Empty entity value must not overwrite the column default: the INSERT
// expression coalesces it to 'RS256' instead of writing ” (GM N8a).
func TestRepository_Create_CoalescesEmptyTokenSigningMethod(t *testing.T) {
	var oauthSQL string
	var oauthArgs []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "app_oauth_config") {
				oauthSQL = sql
				oauthArgs = append([]any(nil), args...)
			}
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}

	require.NoError(t, NewRepository(mockDB).Create(context.Background(), &domain.Application{}))
	require.Contains(t, oauthSQL, "COALESCE(NULLIF($11::varchar, ''), 'RS256')")
	require.Len(t, oauthArgs, 21)
	require.Equal(t, "", oauthArgs[10], "repository passes the raw value; SQL must supply the default")
}
