package products

import (
	"context"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/products"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// StockLedger 是库存原子扣减/归还的可选仓储能力（BE-C-11），由消费方
// （app 层）定义、infra 仓储实现。库存约束必须在 SQL 内以
// `sold = sold + q WHERE sold + q <= quantity` 原子执行：先读后写的
// 校验在并发下单下必然超卖。
type StockLedger interface {
	// IncrementSold 原子消费 quantity 件库存；库存不足/不可售返回 422，
	// 跨租户或不存在返回 NotFound。
	IncrementSold(ctx context.Context, id, organizationID string, quantity int, now time.Time) (*domain.Product, error)
	// ReleaseSold 归还预留库存（未支付订单取消/删除），sold 下限为 0。
	ReleaseSold(ctx context.Context, id, organizationID string, quantity int, now time.Time) (*domain.Product, error)
}

// ReserveBasketInTx 在调用方事务内为一篮商品原子预留库存（BE-C-11）。
// 同一商品出现多次按件数聚合后一次扣减；任一商品库存不足即返回错误，
// 由调用方事务整体回滚，订单与已扣库存都不会留下。
func ReserveBasketInTx(ctx context.Context, tx appuow.UnitOfWork, clk clock.Clock, organizationID string, productIDs []string) error {
	return applyBasketStock(ctx, tx, clk, organizationID, productIDs, true)
}

// ReleaseBasketInTx 在调用方事务内归还一篮商品的预留库存（BE-C-11），
// 用于未支付订单的取消/删除。
func ReleaseBasketInTx(ctx context.Context, tx appuow.UnitOfWork, clk clock.Clock, organizationID string, productIDs []string) error {
	return applyBasketStock(ctx, tx, clk, organizationID, productIDs, false)
}

func applyBasketStock(ctx context.Context, tx appuow.UnitOfWork, clk clock.Clock, organizationID string, productIDs []string, reserve bool) error {
	if len(productIDs) == 0 {
		return nil
	}
	ledger, ok := tx.Products().(StockLedger)
	if !ok {
		return apperrors.NewUnimplemented("atomic product stock ledger is not supported by this dialect")
	}
	quantities := make(map[string]int, len(productIDs))
	order := make([]string, 0, len(productIDs))
	for _, id := range productIDs {
		if _, seen := quantities[id]; !seen {
			order = append(order, id)
		}
		quantities[id]++
	}
	now := clk.Now()
	for _, id := range order {
		var err error
		if reserve {
			_, err = ledger.IncrementSold(ctx, id, organizationID, quantities[id], now)
		} else {
			_, err = ledger.ReleaseSold(ctx, id, organizationID, quantities[id], now)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
