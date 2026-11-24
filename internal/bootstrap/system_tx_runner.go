package bootstrap

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	tentxtx "github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// systemTxRunner 返回一个执行器：把 fn 放进系统作用域事务。
// 供无请求上下文的 Worker（webhook / notification 等）访问 FORCE RLS 表使用（DEF-35）。
func systemTxRunner(uow appuow.UnitOfWork) func(context.Context, func(context.Context, appuow.UnitOfWork) error) error {
	return func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
		return uow.WithinTransaction(tentxtx.System(ctx), fn)
	}
}
