package records

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_Create_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("no rows"))
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.Record{}
	err := repo.Create(context.Background(), entity)
	require.NoError(t, err)
}

func TestRepository_Create_Error(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("db error")
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("no rows"))
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.Record{}
	err := repo.Create(context.Background(), entity)
	require.Error(t, err)
}

// records.user_agent / records.request_id are VARCHAR(255). The repository is
// the last line before the INSERT, and several audit call sites build the
// Record themselves (bypassing the middleware): it must clamp the values, or an
// over-long User-Agent fails the INSERT and rolls back the business
// transaction. Truncation is by rune, so multi-byte characters stay intact.
func TestRepository_Create_TruncatesOversizedEvidenceToColumnWidth(t *testing.T) {
	var insertArgs []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO guliangiam.records") {
				insertArgs = args
			}
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("no rows"))
		},
	}
	originalUserAgent := strings.Repeat("浏览器🚀", 70) // 280 runes
	entity := &domain.Record{
		ID:        "019rec-0000-0000-0000-000000000001",
		Action:    "user.updated",
		UserAgent: originalUserAgent,
		RequestID: strings.Repeat("r", 400),
	}

	err := NewRepository(mockDB).Create(context.Background(), entity)
	require.NoError(t, err, "an over-long User-Agent must not fail the audit INSERT")
	require.Len(t, insertArgs, 18)

	userAgent, ok := insertArgs[7].(string)
	require.True(t, ok)
	assert.Equal(t, 255, utf8.RuneCountInString(userAgent))
	assert.True(t, utf8.ValidString(userAgent), "truncation must not split a multi-byte character")
	assert.Equal(t, []rune(originalUserAgent)[:255], []rune(userAgent))

	requestID, ok := insertArgs[9].(string)
	require.True(t, ok)
	assert.Equal(t, 255, utf8.RuneCountInString(requestID))
	assert.Equal(t, strings.Repeat("r", 255), requestID)

	// The stored Record is clamped too, so the hash chain is computed over the
	// value that actually lands in the row.
	assert.Equal(t, userAgent, entity.UserAgent)
	assert.Equal(t, requestID, entity.RequestID)
}
