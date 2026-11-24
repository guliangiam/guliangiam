package dpop

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestPostgresReplayStoreCheck(t *testing.T) {
	tests := []struct {
		name    string
		tag     pgconn.CommandTag
		execErr error
		want    bool
		wantErr bool
	}{
		{name: "first insert", tag: pgconn.NewCommandTag("INSERT 0 1"), want: true},
		{name: "duplicate", tag: pgconn.NewCommandTag("INSERT 0 0"), want: false},
		{name: "storage error", execErr: errors.New("database unavailable"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &shared.MockDBTX{
				ExecFn: func(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
					require.Equal(t, qInsertJTI, query)
					require.Equal(t, []any{"jti-1"}, args)
					return tt.tag, tt.execErr
				},
			}

			accepted, err := NewPostgresReplayStore(db).Check(context.Background(), "jti-1")
			if tt.wantErr {
				require.Error(t, err)
				appErr := apperrors.FromError(err)
				require.Equal(t, apperrors.CodeUnavailable, appErr.Code)
				require.False(t, accepted)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, accepted)
		})
	}
}
