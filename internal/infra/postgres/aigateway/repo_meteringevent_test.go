package aigateway

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestMeteringEventRepo_Save_UsesIdempotentInsert(t *testing.T) {
	var capturedSQL string
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	repo := NewMeteringEventRepo(mockDB)
	err := repo.Save(context.Background(), &domain.MeteringEvent{ID: "evt-1", OutboxID: "evt-1"})
	require.NoError(t, err)
	require.Contains(t, strings.ToUpper(capturedSQL), "ON CONFLICT (ID) DO NOTHING",
		"audit sink must tolerate outbox retries without duplicate-key failure")
}

func TestMeteringEventRepo_Save_ErrorPropagates(t *testing.T) {
	wantErr := fmt.Errorf("db error")
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, wantErr
		},
	}
	repo := NewMeteringEventRepo(mockDB)
	err := repo.Save(context.Background(), &domain.MeteringEvent{ID: "evt-1", OutboxID: "evt-1"})
	require.ErrorIs(t, err, wantErr)
}

func TestProvidersRepository_CreateAnchorsProviderID(t *testing.T) {
	var capturedArgs []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			require.Equal(t, queryProviderInsert, sql)
			capturedArgs = args
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	repo := NewProvidersRepository(mockDB)
	provider := &domain.LlmProvider{ID: "provider-1"}
	require.NoError(t, repo.Create(context.Background(), provider))
	require.Len(t, capturedArgs, 22)
	require.Equal(t, provider.ID, capturedArgs[2])
}

func TestMeteringEventRepo_Save_EmptyIDRejected(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			t.Fatal("Exec must not be called for invalid input")
			return pgconn.CommandTag{}, nil
		},
	}
	repo := NewMeteringEventRepo(mockDB)
	require.Error(t, repo.Save(context.Background(), &domain.MeteringEvent{}))
}
