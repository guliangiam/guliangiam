package organizations

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// runInTx executes fn inside a transaction when the injected UoW supports it.
// An unchecked type assertion used to panic here whenever an alternative UoW
// implementation did not expose TxRunner (BE-A-19); the missing capability is
// now an explicit internal error instead of a process crash.
func runInTx(ctx context.Context, unitOfWork any, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	runner, ok := appuow.EnsureTxRunner(unitOfWork)
	if !ok {
		return apperrors.NewInternal("unit of work does not support transactions")
	}
	return runner.WithinTransaction(ctx, fn)
}
