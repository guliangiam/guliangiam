package orders

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	appcoupons "github.com/guliangiam/guliangiam/internal/app/coupons"
	appproducts "github.com/guliangiam/guliangiam/internal/app/products"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	coupondomain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
	"github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// CreatePaymentFunc is the signature for creating a payment via the payment gateway.
// It is wired to the actual payment infrastructure in the bootstrap layer.
type CreatePaymentFunc func(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (paymentURL, tradeNo string, err error)

// QueryPaymentFunc is the signature for querying a payment status from the payment gateway.
// It is wired to the actual payment infrastructure in the bootstrap layer, which is also
// where the channel's own status vocabulary is resolved into a domain Outcome — the app
// layer never sees a third-party status string.
type QueryPaymentFunc func(ctx context.Context, outTradeNo string) (outcome payments.Outcome, err error)

// paymentAdapters 是 SetPaymentAdapters 写入的共享适配器快照（BE-C-21）。
// 用 atomic.Pointer 整体替换取代两个裸包级可变函数变量：并发测试/装配
// 改写不再构成数据竞争，读取侧永远看到一致的 (create, query) 组合。
type paymentAdapters struct {
	create CreatePaymentFunc
	query  QueryPaymentFunc
}

var sharedPaymentAdapters atomic.Pointer[paymentAdapters]

func defaultCreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (paymentURL, tradeNo string, err error) {
	return "", "", apperrors.NewUnavailable("payment gateway not configured: createPaymentFn not wired")
}

func defaultQueryPayment(ctx context.Context, outTradeNo string) (payments.Outcome, error) {
	return payments.OutcomeUnknown, apperrors.NewUnavailable("payment gateway not configured: queryPaymentFn not wired")
}

// SetPaymentAdapters wires the payment adapter functions.
// This should be called from the bootstrap layer to connect the app layer
// to the actual payment gateway implementations without creating a direct import dependency.
// 优先使用 Service 字段注入（CreatePayment/QueryPayment）；本入口保留给
// 组合根的全局装配，写入是原子的整体快照替换。
func SetPaymentAdapters(createFn CreatePaymentFunc, queryFn QueryPaymentFunc) {
	next := paymentAdapters{create: defaultCreatePayment, query: defaultQueryPayment}
	if prev := sharedPaymentAdapters.Load(); prev != nil {
		next = *prev
	}
	if createFn != nil {
		next.create = createFn
	}
	if queryFn != nil {
		next.query = queryFn
	}
	sharedPaymentAdapters.Store(&next)
}

// createPayment 解析本 Service 的支付创建适配器：字段注入优先，
// 其次组合根装配的共享快照，最后 fail-closed 默认实现。
func (s Service) createPayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnURL string) (string, string, error) {
	if s.CreatePayment != nil {
		return s.CreatePayment(ctx, orderID, amount, currency, subject, notifyURL, returnURL)
	}
	if a := sharedPaymentAdapters.Load(); a != nil && a.create != nil {
		return a.create(ctx, orderID, amount, currency, subject, notifyURL, returnURL)
	}
	return defaultCreatePayment(ctx, orderID, amount, currency, subject, notifyURL, returnURL)
}

// queryPayment 解析本 Service 的支付查询适配器，优先级同 createPayment。
func (s Service) queryPayment(ctx context.Context, outTradeNo string) (payments.Outcome, error) {
	if s.QueryPayment != nil {
		return s.QueryPayment(ctx, outTradeNo)
	}
	if a := sharedPaymentAdapters.Load(); a != nil && a.query != nil {
		return a.query(ctx, outTradeNo)
	}
	return defaultQueryPayment(ctx, outTradeNo)
}

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.CommerceUoW
	// CreatePayment/QueryPayment 是可选的按实例注入点（BE-C-21）：
	// 测试与多网关装配直接挂在 Service 上，不再改写包级变量。
	CreatePayment CreatePaymentFunc
	QueryPayment  QueryPaymentFunc
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.UnitOfWork) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}

type ListOrdersQuery struct {
	Page, PageSize                int
	OrganizationID, Keyword, User string
	State                         *string
}
type CreateOrderCommand struct {
	OrganizationID, Name, DisplayName, User string
	Products                                []string
	ProductInfos                            []domain.ProductInfo
	Payment                                 string
	Price                                   float64
	Currency, CouponName                    string
	CouponDiscount                          float64
	NotifyURL                               string
	ReturnURL                               string
}
type PatchOrderCommand struct {
	ID                            string
	DisplayName, Payment, Message *string
	Price                         *float64
	Currency                      *string
	CouponName                    *string
	CouponDiscount                *float64
	// State is an optional lifecycle move. It is not a free-text column write:
	// PatchOrder routes it through domain.Order.TransitionTo, so only edges the
	// registry declares are accepted (设计报告 42.3.7).
	State    *string
	Revision int64
}

type OrderListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	User           string    `json:"user,omitempty"`
	Price          float64   `json:"price"`
	Currency       string    `json:"currency,omitempty"`
	State          string    `json:"state"`
	PaymentURL     string    `json:"paymentUrl,omitempty"`
	TradeNo        string    `json:"tradeNo,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type OrderDetail struct {
	OrderListItem
	Products       []string             `json:"products,omitempty"`
	ProductInfos   []domain.ProductInfo `json:"productInfos,omitempty"`
	Payment        string               `json:"payment,omitempty"`
	Message        string               `json:"message,omitempty"`
	CouponName     string               `json:"couponName,omitempty"`
	CouponDiscount float64              `json:"couponDiscount"`
	Revision       int64                `json:"revision"`
	// PaidAt is the settlement instant recorded when the state machine entered
	// `paid`, and is the basis that makes a "paid" conclusion self-verifiable
	// afterwards (42.3.3). Nil means the order has never reached `paid`.
	PaidAt *time.Time `json:"paidAt,omitempty"`
	// Terminal / AwaitingPayment / PollAfterSeconds are the server's polling
	// guidance for this order (42.3.11 条 2, AC-FE-016). They are derived here
	// from the state machine registry so a polling client never has to decide
	// for itself whether to keep polling, when to stop, or how often to ask —
	// a client that re-derives those from the state string is a second, drifting
	// copy of the transition graph.
	Terminal         bool `json:"terminal"`
	AwaitingPayment  bool `json:"awaitingPayment"`
	PollAfterSeconds int  `json:"pollAfterSeconds"`
}

func toListItem(o *domain.Order) OrderListItem {
	return OrderListItem{ID: o.ID, OrganizationID: o.OrganizationID, Name: o.Name, DisplayName: o.DisplayName,
		User: o.User, Price: o.Price, Currency: o.Currency, State: o.State,
		PaymentURL: o.PaymentURL, TradeNo: o.TradeNo,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt}
}
func toDetail(o *domain.Order) OrderDetail {
	infos := make([]domain.ProductInfo, len(o.ProductInfos))
	copy(infos, o.ProductInfos)
	return OrderDetail{OrderListItem: toListItem(o), Products: append([]string(nil), o.Products...),
		ProductInfos: infos, Payment: o.Payment, Message: o.Message,
		CouponName: o.CouponName, CouponDiscount: o.CouponDiscount, Revision: o.Revision,
		PaidAt:           o.PaidAt,
		Terminal:         domain.IsTerminal(o.State),
		AwaitingPayment:  domain.IsAwaitingPayment(o.State),
		PollAfterSeconds: int(domain.PollAfter(o.State) / time.Second),
	}
}

func (s Service) ListOrders(ctx context.Context, q ListOrdersQuery) ([]OrderListItem, int64, error) {
	items, total, err := s.UOW.Orders().List(ctx, domain.ListQuery(q))
	if err != nil {
		return nil, 0, err
	}
	r := make([]OrderListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func (s Service) GetOrder(ctx context.Context, organizationID, id string) (*OrderDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	o, err := s.UOW.Orders().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	if err := ensureOrderTenantScope(o, organizationID); err != nil {
		return nil, err
	}
	d := toDetail(o)
	return &d, nil
}
func (s Service) CreateOrder(ctx context.Context, c CreateOrderCommand) error {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	infos := make([]domain.ProductInfo, len(c.ProductInfos))
	copy(infos, c.ProductInfos)
	o := &domain.Order{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		User: c.User, Products: append([]string(nil), c.Products...), ProductInfos: infos,
		Payment: c.Payment, Price: c.Price, Currency: c.Currency, State: domain.StateCreated,
		CouponName: c.CouponName, CouponDiscount: c.CouponDiscount,
		CreatedAt: now, UpdatedAt: now, Revision: 1}
	if err := o.Validate(); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Orders().Create(ctx, o); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			o.OrganizationID, o.User,
			"order.created", "Order", o.ID,
			"Order", "order.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) PatchOrder(ctx context.Context, organizationID string, c PatchOrderCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation, so a cross-tenant request
		// leaves no write and no audit record behind.
		o, err := tx.Orders().GetByIDForOrganization(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if err := ensureOrderTenantScope(o, organizationID); err != nil {
			return err
		}
		if c.Revision > 0 && o.Revision != c.Revision {
			return apperrors.NewConflict("order revision mismatch")
		}
		if c.DisplayName != nil {
			o.DisplayName = *c.DisplayName
		}
		if c.Payment != nil {
			o.Payment = *c.Payment
		}
		if c.Message != nil {
			o.Message = *c.Message
		}
		if c.Price != nil {
			o.Price = *c.Price
		}
		if c.Currency != nil {
			o.Currency = *c.Currency
		}
		if c.CouponName != nil {
			o.CouponName = *c.CouponName
		}
		if c.CouponDiscount != nil {
			o.CouponDiscount = *c.CouponDiscount
		}
		if c.State != nil {
			if err := o.TransitionTo(*c.State); err != nil {
				return err
			}
			s.stampPaidAt(o)
		}
		// PATCH 后的聚合必须整体重新过一遍领域不变量（Price>=0、
		// CouponDiscount>=0、状态合法），否则负价/负折扣会绕过 Create 路径
		// 的校验直接落库（BE-C-02）。
		if err := o.Validate(); err != nil {
			return err
		}
		o.UpdatedAt = s.Clock.Now()
		o.Revision++
		if err := tx.Orders().Update(ctx, o); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			o.OrganizationID, o.User,
			"order.updated", "Order", o.ID,
			"Order", "order.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) DeleteOrder(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the audit
		// row carries the entity's real organization.
		o, err := tx.Orders().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := ensureOrderTenantScope(o, organizationID); err != nil {
			return err
		}
		// BE-C-11: deleting an unpaid catalog order returns its reserved
		// stock; a paid order's consumption is final.
		if o.PaidAt == nil && len(o.ProductInfos) > 0 {
			if err := appproducts.ReleaseBasketInTx(ctx, tx, s.Clock, organizationID, o.Products); err != nil {
				return err
			}
		}
		if err := tx.Orders().Delete(ctx, id, organizationID); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			o.OrganizationID, o.User,
			"order.deleted", "Order", o.ID,
			"Order", "order.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

// PlaceOrder creates an order with validated product/pricing/coupon info.
//
// Everything the order will claim about itself — which products it is made of,
// what they are worth and what currency they are in — is read from the products
// aggregate inside the same transaction that writes the order. The caller
// supplies identifiers and, at most, a price to be checked against the catalog;
// it never supplies the snapshot. That is what makes an order's own history
// trustworthy after the fact (§42.3.3) and what closes the price-tampering path
// where a buyer posts a cheaper amount than the catalog charges.
func (s Service) PlaceOrder(ctx context.Context, c CreateOrderCommand) (*OrderDetail, error) {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if len(c.Products) == 0 {
		return nil, apperrors.NewInvalidArgument("at least one product is required")
	}

	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now()

	o := &domain.Order{
		ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		User: c.User, Products: append([]string(nil), c.Products...),
		Payment:    c.Payment,
		State:      domain.StateCreated,
		CouponName: c.CouponName,
		// CouponDiscount is never taken from the caller: it is computed from
		// the validated coupon below (BE-C-07). A client-supplied discount
		// without a coupon code is dropped, not trusted.
		CouponDiscount: 0,
		CreatedAt:      now, UpdatedAt: now, Revision: 1,
	}

	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		pricing, err := resolveCatalog(ctx, tx, c.OrganizationID, c.Products, c.Price, c.Currency)
		if err != nil {
			return err
		}
		o.ProductInfos = pricing.Infos
		o.Price = pricing.Price
		o.Currency = pricing.Currency
		// BE-C-11: stock is reserved atomically (sold = sold + q under the
		// sold+q <= quantity guard) inside the order transaction. The stale
		// read-check in BuyProduct is advisory only — this is the hard gate
		// that makes concurrent checkouts of the last unit fail instead of
		// overselling. A rejection rolls the whole order back.
		if err := appproducts.ReserveBasketInTx(ctx, tx, s.Clock, c.OrganizationID, c.Products); err != nil {
			return err
		}
		// BE-C-07: the coupon is validated, priced and consumed inside the same
		// transaction that writes the order. The discount is capped at the
		// catalog total, so the payable amount below is the discounted price.
		var appliedCoupon *coupondomain.Coupon
		if strings.TrimSpace(c.CouponName) != "" {
			names := make([]string, 0, len(pricing.Infos))
			for _, info := range pricing.Infos {
				names = append(names, info.Name)
			}
			discount, coupon, cerr := appcoupons.ResolveCheckoutCoupon(ctx, tx, s.Clock, c.OrganizationID, c.CouponName, c.User, names, pricing.Price)
			if cerr != nil {
				return cerr
			}
			appliedCoupon = coupon
			o.CouponDiscount = discount
			o.Price = pricing.Price - discount
		}
		if strings.TrimSpace(o.Name) == "" {
			o.Name = catalogOrderName(pricing.Infos, id)
		}
		if strings.TrimSpace(o.DisplayName) == "" {
			o.DisplayName = catalogOrderDisplayName(pricing.Infos, o.Name)
		}
		if err := o.Validate(); err != nil {
			return err
		}
		if err := tx.Orders().Create(ctx, o); err != nil {
			return err
		}
		if appliedCoupon != nil {
			if err := appcoupons.RecordUsageInTx(ctx, tx, s.Clock, s.UUID, appliedCoupon, c.User, o.ID); err != nil {
				return err
			}
		}
		// BE-C-17: the buyer's money-moving creation carries the same audit
		// chain as every other order write, committed with the order itself.
		return s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			o.OrganizationID, o.User,
			"order.created", "Order", o.ID,
			"Order", "order.created",
			map[string]any{},
		)
	}); err != nil {
		return nil, err
	}

	// Ask the channel for a trade. `payment_pending` is the registry's state for
	// "the trade exists and the buyer has not paid yet"; the legacy code stored
	// `Pending` here, which is not a state in the registry at all and made
	// `domain.Order.Validate` reject its own writes.
	subject := o.DisplayName
	if subject == "" {
		subject = o.Name
	}
	paymentURL, tradeNo, err := s.createPayment(ctx, o.ID, o.Price, o.Currency, subject, c.NotifyURL, c.ReturnURL)
	if err != nil {
		// The channel never produced a trade, so the order is exactly where it
		// was: `created`. Staying there is not a swallow — `created` is the
		// registry's state for "no payment intent yet", and the retry action
		// (`:pay`) re-initiates from it. The failure is recorded as the order's
		// message so the page can explain itself, and the write error is
		// propagated instead of being discarded.
		slog.WarnContext(ctx, "payment gateway call failed",
			"order_id", o.ID, "error", err)
		if err := s.recordOrderMessage(ctx, o, "payment gateway unavailable"); err != nil {
			return nil, err
		}
	} else {
		o.PaymentURL = paymentURL
		o.TradeNo = tradeNo
		if err := s.applyOrderState(ctx, o, domain.StatePaymentPending, ""); err != nil {
			return nil, err
		}
	}

	d := toDetail(o)
	return &d, nil
}

// recordOrderMessage persists a non-lifecycle write: the state machine stays
// where it is, only the explanation changes. Callers reach for it when the
// channel failed before it could produce a trade — the order has no state
// transition to make, but the reader still needs to be told what happened.
func (s Service) recordOrderMessage(ctx context.Context, o *domain.Order, message string) error {
	o.Message = message
	o.UpdatedAt = s.Clock.Now()
	o.Revision++
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return tx.Orders().Update(ctx, o)
	})
}

// stampPaidAt records the settlement instant that entering a §1.1 paid state
// presupposes. `ck_orders_paid_at` makes the column and the state a
// biconditional, so this belongs to the transition itself rather than to any
// individual writer — PatchOrder and applyOrderState both call it for that
// reason.
func (s Service) stampPaidAt(o *domain.Order) {
	if domain.RequiresPaidAt(o.State) && o.PaidAt == nil {
		paidAt := s.Clock.Now()
		o.PaidAt = &paidAt
	}
}

// applyOrderState persists a lifecycle transition through the registry's
// transition table. Every state write in this package goes through it: the
// transition is validated before the write and the update error is propagated,
// so a rejected transition can never be silently swallowed and leave the order
// looking payable after the channel already refused it. The edge rule itself
// lives in domain.Order.TransitionTo, not here, so there is one copy of it.
func (s Service) applyOrderState(ctx context.Context, o *domain.Order, state, message string) error {
	if o.State == state {
		return nil
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := o.TransitionTo(state); err != nil {
			return err
		}
		s.stampPaidAt(o)
		switch {
		case message != "":
			o.Message = message
		case state == domain.StatePaymentPending:
			// A fresh attempt supersedes the previous failure explanation;
			// leaving it behind would make the page contradict the state.
			o.Message = ""
		}
		o.UpdatedAt = s.Clock.Now()
		o.Revision++
		return tx.Orders().Update(ctx, o)
	})
}

// CancelOrder cancels an order that has not collected money yet. Ownership is
// decided by the scoped read in the same transaction as the write, so a
// cross-tenant id is reported as missing before any transition is persisted.
func (s Service) CancelOrder(ctx context.Context, organizationID, id string, revision int64) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		o, err := tx.Orders().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := ensureOrderTenantScope(o, organizationID); err != nil {
			return err
		}
		if revision > 0 && o.Revision != revision {
			return apperrors.NewConflict("order revision mismatch")
		}
		// 已收款订单不能走取消，必须走退款（payment_refund_required，422）。
		if o.PaidAt != nil {
			return apperrors.NewWithMeta(
				apperrors.CodePaymentRefundRequired,
				"paid order must be refunded, not cancelled",
				map[string]any{"field": "state", "value": o.State},
			)
		}
		if err := o.TransitionTo(domain.StateCancelled); err != nil {
			return err
		}
		// BE-C-11: a cancelled order never collected money, so the stock it
		// reserved at placement goes back to the catalog in the same
		// transaction. Only catalog-placed orders carry ProductInfos.
		if o.PaidAt == nil && len(o.ProductInfos) > 0 {
			if err := appproducts.ReleaseBasketInTx(ctx, tx, s.Clock, organizationID, o.Products); err != nil {
				return err
			}
		}
		o.UpdatedAt = s.Clock.Now()
		o.Revision++
		return tx.Orders().Update(ctx, o)
	})
}

// PayOrder advances an unpaid order towards settlement by talking to the
// channel, and never marks it paid on the caller's word alone.
//
// The caller is either the browser returning from the channel (`:notify-paid`)
// or an operator pressing "recheck" (`:pay`). Both must produce the same result,
// so both go through here: an order with no trade yet gets a trade created, and
// an order with a trade gets that trade queried. Only a channel-confirmed
// success moves the order to `paid`, and `paid` is only ever recorded together
// with `paid_at` (`ck_orders_paid_at`).
func (s Service) PayOrder(ctx context.Context, organizationID, id, paymentRef string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	// Ownership is settled inside a transaction boundary before the channel is
	// contacted, and that transaction closes before the network call: a slow
	// gateway must not keep the order's transaction open. The scoped read is
	// the only lookup this path performs, so a foreign id can never reach the
	// channel or a state write.
	var o *domain.Order
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		found, err := tx.Orders().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := ensureOrderTenantScope(found, organizationID); err != nil {
			return err
		}
		o = found
		return nil
	}); err != nil {
		return err
	}
	if paymentRef != "" {
		o.Payment = paymentRef
	}

	switch {
	case o.State == domain.StatePaid || o.State == domain.StateCompleted ||
		o.State == domain.StateRefunded || o.State == domain.StateClosed:
		// Already settled: repeating the action must not produce a second
		// payment side effect (AC-FE-016「不产生重复支付副作用」).
		return nil
	case !domain.IsAwaitingPayment(o.State) && o.State != domain.StateFailed:
		return validatePayableState(o.State)
	}

	if o.TradeNo == "" {
		// Nothing was ever submitted to the channel, so there is nothing to
		// query. Create the trade; the legacy code marked such an order paid
		// without any channel confirmation at all.
		subject := o.DisplayName
		if subject == "" {
			subject = o.Name
		}
		paymentURL, tradeNo, err := s.createPayment(ctx, o.ID, o.Price, o.Currency, subject, "", "")
		if err != nil {
			slog.WarnContext(ctx, "payment gateway call failed during retry",
				"order_id", o.ID, "error", err)
			// The state stays where it is — `failed` already says the attempt
			// was rejected, `created` has no declared edge to `failed` — and
			// only the explanation is refreshed.
			return s.recordOrderMessage(ctx, o, "payment gateway unavailable")
		}
		o.PaymentURL = paymentURL
		o.TradeNo = tradeNo
		return s.applyOrderState(ctx, o, domain.StatePaymentPending, "")
	}

	gatewayOutcome, err := s.queryPayment(ctx, o.TradeNo)
	if err != nil {
		slog.WarnContext(ctx, "payment gateway query failed",
			"order_id", o.ID, "trade_no", o.TradeNo, "error", err)
		return apperrors.NewUnavailable("failed to verify payment status with gateway")
	}
	return s.applyGatewayOutcome(ctx, o, gatewayOutcome, "")
}

// applyGatewayOutcome maps a channel verdict onto the order lifecycle. A
// verdict whose target state is unreachable from the current state is ignored
// rather than forced: callbacks arrive out of order and are retried, so a stale
// duplicate must be a no-op, never an error and never a write.
func (s Service) applyGatewayOutcome(ctx context.Context, o *domain.Order, outcome payments.Outcome, raw string) error {
	var target, message string
	switch outcome {
	case payments.OutcomeSucceeded:
		target = domain.StatePaid
	case payments.OutcomePending:
		target = domain.StatePaymentPending
	case payments.OutcomeFailed:
		target = domain.StateFailed
		message = "payment rejected by gateway"
		if raw != "" {
			message += ": " + raw
		}
	case payments.OutcomeCancelled:
		target = domain.StateCancelled
	case payments.OutcomeRefunded:
		target = domain.StateRefunded
	case payments.OutcomeUnknown:
		slog.WarnContext(ctx, "unrecognized gateway trade status, leaving order state untouched",
			"order_id", o.ID, "trade_no", o.TradeNo, "trade_status", raw)
		return nil
	}
	if o.State == target {
		return nil
	}
	if !o.CanTransitionTo(target) {
		slog.InfoContext(ctx, "ignoring out-of-order gateway verdict for order",
			"order_id", o.ID, "trade_no", o.TradeNo,
			"current_state", o.State, "target_state", target, "trade_status", raw)
		return nil
	}
	return s.applyOrderState(ctx, o, target, message)
}

// GetUserOrders lists orders for a specific user.
//
// The caller identity is passed explicitly by the transport layer (app must not
// import transport). A caller may always read its own orders; reading another
// user's orders requires a platform administrator scope that covers the tenant.
// Without such a scope the request fails closed with 403 instead of returning
// another user's data (same-tenant horizontal privilege escalation).
func (s Service) GetUserOrders(ctx context.Context, orgID, user, actorUserID string, page, pageSize int) ([]OrderListItem, int64, error) {
	if err := authorizeCrossUserRead(ctx, orgID, user, actorUserID); err != nil {
		return nil, 0, err
	}
	items, total, err := s.UOW.Orders().List(ctx, domain.ListQuery{
		Page: page, PageSize: pageSize, OrganizationID: orgID, User: user,
	})
	if err != nil {
		return nil, 0, err
	}
	r := make([]OrderListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}

// authorizeCrossUserRead enforces ownership on user-scoped read endpoints.
//
// Self reads always pass. Cross-user reads require a platform administrator
// scope covering the trusted tenant; that classification is resolved by
// ScopeAuthorizerMiddleware and reused here (no new permission point). Any
// unresolvable identity or tenant denies (fail-closed), so a denied caller
// never reaches the repository.
func authorizeCrossUserRead(ctx context.Context, organizationID, targetUserID, actorUserID string) error {
	actor := strings.TrimSpace(actorUserID)
	if actor == "" {
		return apperrors.NewForbidden("caller identity is required to read user-scoped commerce data")
	}
	if actor == targetUserID {
		return nil
	}
	// Mode==System is only reachable from trusted internal callers (Worker/Cron
	// entry points wrap ctx with TenantFilterSystem). Since OPEN-API-42 C1 an
	// authorized explicit cross-tenant request injects TenantFilterCross plus
	// the admin scope classification, so platform-admin reads are honoured by
	// the scope.Covers check below instead of a filter-mode shortcut.
	if tenantctx.TenantScopeFromContext(ctx).Mode == tenantctx.TenantFilterSystem {
		return nil
	}
	tenantID, err := uuid.Parse(strings.TrimSpace(organizationID))
	if err != nil {
		return apperrors.NewForbidden("cross-user access requires a resolvable tenant")
	}
	scope, ok := scopeauthorizer.AdminScopeFromContext(ctx)
	if ok && scope.Covers(tenantID) {
		return nil
	}
	return apperrors.NewForbidden("cross-user access requires a platform administrator scope covering the tenant")
}

// NotifyOrderPayment applies a channel callback to the order lifecycle. It
// receives the channel's own status string together with the provider, and
// translates it here: the transport layer has no business knowing what a state
// literal is, and the same channel answer must mean the same thing whichever
// surface handled it.
//
// amount and currency come from the signature-verified channel payload and are
// bound to the local order before any state transition: a callback that
// confirms a different amount (or currency) than the order was created with is
// rejected with a warning instead of silently marking the order paid. Repeat
// deliveries of the same event are no-ops — the state machine only moves an
// order that is still awaiting payment.
func (s Service) NotifyOrderPayment(ctx context.Context, tradeNo, providerType, tradeStatus string, amount float64, currency string) error {
	o, err := s.UOW.Orders().GetByTradeNo(ctx, tradeNo)
	if err != nil {
		return err
	}
	outcome := payments.TranslateTradeStatus(providerType, tradeStatus)
	if outcome == payments.OutcomeUnknown {
		slog.WarnContext(ctx, "ignoring payment notify with unrecognized trade status",
			"order_id", o.ID, "trade_no", tradeNo,
			"provider_type", providerType, "trade_status", tradeStatus)
		return nil
	}
	if !domain.IsAwaitingPayment(o.State) {
		// A duplicate callback for an order that already moved on must not
		// produce a second side effect on it.
		slog.InfoContext(ctx, "ignoring payment notify for order that is no longer awaiting payment",
			"order_id", o.ID, "trade_no", tradeNo, "current_state", o.State)
		return nil
	}
	if err := verifyCallbackBinding(ctx, o, outcome, amount, currency); err != nil {
		return err
	}
	return s.applyGatewayOutcome(ctx, o, outcome, tradeStatus)
}

// verifyCallbackBinding binds the verified amount/currency of a callback to the
// local order. A mismatch is a warning plus a rejected state transition — the
// gateway can retry forever for all we care, the order must never be marked
// paid for money it did not ask for. Partial refunds legitimately carry a
// smaller amount than the order, so they are exempt from the amount check.
func verifyCallbackBinding(ctx context.Context, o *domain.Order, outcome payments.Outcome, amount float64, currency string) error {
	if outcome == payments.OutcomeRefunded {
		return nil
	}
	if outcome == payments.OutcomeSucceeded && amount <= 0 {
		slog.WarnContext(ctx, "rejecting payment confirmation without a verifiable amount",
			"order_id", o.ID, "trade_no", o.TradeNo)
		return apperrors.NewInvalidArgument("payment callback carries no verifiable amount")
	}
	const amountTolerance = 0.01
	if amount > 0 && (amount < o.Price-amountTolerance || amount > o.Price+amountTolerance) {
		slog.WarnContext(ctx, "payment callback amount does not match order, rejecting state transition",
			"order_id", o.ID, "trade_no", o.TradeNo,
			"order_price", o.Price, "callback_amount", amount)
		return validateCallbackAmount(amount, o.Price)
	}
	if currency != "" && o.Currency != "" && !strings.EqualFold(currency, o.Currency) {
		slog.WarnContext(ctx, "payment callback currency does not match order, rejecting state transition",
			"order_id", o.ID, "trade_no", o.TradeNo,
			"order_currency", o.Currency, "callback_currency", currency)
		return validateCallbackCurrency(currency, o.Currency)
	}
	return nil
}

// 安全边界：PEP 中间件（APIAuthorizationMiddleware）已通过 Casbin 验证了权限。
// RLS 提升只在 service 内部生效（事务内 LOCAL set_config），事务结束自动清除。
