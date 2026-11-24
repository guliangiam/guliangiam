package dpop

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestMySQLReplayStoreCheck(t *testing.T) {
	tests := []struct {
		name    string
		rows    int64
		execErr error
		want    bool
		wantErr bool
	}{
		{name: "first insert", rows: 1, want: true},
		{name: "duplicate", rows: 0, want: false},
		{name: "storage error", execErr: errors.New("database unavailable"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			expectation := mock.ExpectExec(regexp.QuoteMeta(qInsertJTI)).WithArgs("jti-1")
			if tt.execErr != nil {
				expectation.WillReturnError(tt.execErr)
			} else {
				expectation.WillReturnResult(sqlmock.NewResult(0, tt.rows))
			}

			accepted, err := NewMySQLReplayStore(db).Check(context.Background(), "jti-1")
			if tt.wantErr {
				require.Error(t, err)
				appErr := apperrors.FromError(err)
				require.Equal(t, apperrors.CodeUnavailable, appErr.Code)
				require.False(t, accepted)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, accepted)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

type resultWithRowsAffectedError struct{}

func (resultWithRowsAffectedError) LastInsertId() (int64, error) { return 0, nil }

func (resultWithRowsAffectedError) RowsAffected() (int64, error) {
	return 0, errors.New("affected rows unavailable")
}

type replayStoreTestDB struct {
	*sql.DB
	result sql.Result
}

func (db replayStoreTestDB) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return db.result, nil
}

func TestMySQLReplayStoreCheckRowsAffectedErrorFailsClosed(t *testing.T) {
	db := replayStoreTestDB{result: resultWithRowsAffectedError{}}
	accepted, err := NewMySQLReplayStore(db).Check(context.Background(), "jti-1")
	require.Error(t, err)
	require.False(t, accepted)
	require.Equal(t, apperrors.CodeUnavailable, apperrors.FromError(err).Code)
}
