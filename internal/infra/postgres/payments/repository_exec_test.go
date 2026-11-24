package payments

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fixedReceiptTime = time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)

func TestPayments_NewRepository(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	assert.NotNil(t, repo)
}

func TestPayments_Create(t *testing.T) {
	var capturedSQL string
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(db)

	item := &domain.Payment{ID: "test-1"}
	err := repo.Create(t.Context(), item)
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "INSERT")
}

func TestPayments_Create_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	item := &domain.Payment{ID: "test-1"}
	err := repo.Create(t.Context(), item)
	assert.Error(t, err)
}

func TestPayments_Update(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = args
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(db)

	item := &domain.Payment{
		ID: "test-1", OrganizationID: "org-1",
		ChannelOrderNo: "CH-001", AttemptCount: 3,
		LastErrorCode: "E_TIMEOUT", LastErrorMessage: "gateway timeout",
	}
	err := repo.Update(t.Context(), item)
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "UPDATE")
	assert.Contains(t, capturedSQL, "organization_id = guliangiam.resolve_org_id($14)")
	assert.Contains(t, capturedSQL, "channel_order_no=$15")
	assert.Contains(t, capturedSQL, "attempt_count=$16")
	assert.Contains(t, capturedSQL, "last_error_code=$17")
	assert.Contains(t, capturedSQL, "last_error_message=$18")
	require.Len(t, capturedArgs, 18)
	assert.Equal(t, "org-1", capturedArgs[13])
	assert.Equal(t, "CH-001", capturedArgs[14])
	assert.Equal(t, 3, capturedArgs[15])
	assert.Equal(t, "E_TIMEOUT", capturedArgs[16])
	assert.Equal(t, "gateway timeout", capturedArgs[17])
}

func TestPayments_Update_ZeroRowsIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.Update(t.Context(), &domain.Payment{ID: "test-1", OrganizationID: "org-1"})
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestPayments_Update_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	item := &domain.Payment{ID: "test-1"}
	err := repo.Update(t.Context(), item)
	assert.Error(t, err)
}

func TestPayments_Delete(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL = sql
			capturedArgs = args
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.Delete(t.Context(), "test-1", "org-1")
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "DELETE")
	assert.Contains(t, capturedSQL, "organization_id = guliangiam.resolve_org_id($2)")
	require.Len(t, capturedArgs, 2)
	assert.Equal(t, "test-1", capturedArgs[0])
	assert.Equal(t, "org-1", capturedArgs[1])
}

func TestPayments_Delete_ForeignOrganizationIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 0"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.Delete(t.Context(), "test-1", "org-1")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestPayments_Delete_Error(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, assert.AnError
		},
	}
	repo := NewRepository(db)

	err := repo.Delete(t.Context(), "test-1", "org-1")
	assert.Error(t, err)
}

func TestCallbackReceiptsRepository_CreateUsesAuditFields(t *testing.T) {
	var capturedArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			assert.Equal(t, ReceiptInsert, sql)
			capturedArgs = args
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewCallbackReceiptsRepository(db)
	rec := &domain.CallbackReceipt{
		ID: "receipt-1", OrganizationID: "org-1", Channel: "alipay",
		PayloadSHA256: "sha256", Payload: []byte("payload"),
		ReceivedAt: fixedReceiptTime, ProcessState: domain.ReceiptStateReceived,
		CreatedAt: fixedReceiptTime, UpdatedAt: fixedReceiptTime.Add(time.Second), Revision: 1,
	}
	require.NoError(t, repo.Create(t.Context(), rec))
	require.Len(t, capturedArgs, 16)
	assert.Equal(t, rec.CreatedAt, capturedArgs[13])
	assert.Equal(t, rec.UpdatedAt, capturedArgs[14])
	assert.Equal(t, int64(1), capturedArgs[15])
	assert.NotEqual(t, rec.ID, capturedArgs[15])
}
