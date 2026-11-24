package uow

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

// 嵌套 WithinTransaction 在 MySQL 上用 SAVEPOINT 实现独立回滚边界。
// 这两条用例锁定该语义：内层成功必须 RELEASE，内层失败必须 ROLLBACK TO 且外层仍可决定提交。
func TestWithinTransaction_NestedUsesSavepoint(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(`SAVEPOINT uow_sp_[0-9]+`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`RELEASE SAVEPOINT uow_sp_[0-9]+`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	u := New(db)
	err = u.WithinTransaction(context.Background(), func(ctx context.Context, outer appuow.UnitOfWork) error {
		require.NotNil(t, outer)
		return outer.WithinTransaction(ctx, func(context.Context, appuow.UnitOfWork) error { return nil })
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithinTransaction_NestedFailureRollsBackToSavepoint(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	innerErr := errors.New("inner step failed")
	mock.ExpectBegin()
	mock.ExpectExec(`SAVEPOINT uow_sp_[0-9]+`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`ROLLBACK TO SAVEPOINT uow_sp_[0-9]+`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	u := New(db)
	err = u.WithinTransaction(context.Background(), func(ctx context.Context, outer appuow.UnitOfWork) error {
		inner := outer.WithinTransaction(ctx, func(context.Context, appuow.UnitOfWork) error { return innerErr })
		require.ErrorIs(t, inner, innerErr)
		return inner
	})
	require.ErrorIs(t, err, innerErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
