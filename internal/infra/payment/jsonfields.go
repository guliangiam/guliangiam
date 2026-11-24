package payment

// 第三方网关请求/响应中反复出现的 JSON 字段名。同一名字散落成多个字面量时，任何
// 一次改名都可能漏改其中一处，而漏改只会在真实支付往返时才会暴露；集中成常量后
// 由编译器保证一致。
const (
	fieldAmount     = "amount"
	fieldCurrency   = "currency"
	fieldOrderID    = "order_id"
	fieldOutTradeNo = "out_trade_no"
	fieldReason     = "reason"
	fieldValue      = "value"
)
