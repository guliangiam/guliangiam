package payment

// 本系统对外暴露的支付/退款状态词汇。各网关的 mapXxxStatus 都归一化到这一组
// 值，下游订单状态机按字面值比对；提取成常量只为消除散落的重复字面量，值与
// 历史行为完全一致（改动值等于改动语义）。
const (
	statusPending    = "pending"
	statusPaid       = "paid"
	statusFailed     = "failed"
	statusSuccess    = "success"
	statusRefunded   = "refunded"
	statusProcessing = "processing"
)

// 渠道负载里的状态词。大小写是渠道协议的一部分（Adyen/PayPal 等全大写，
// Paddle/Polar 等小写），映射前必须按原样比对；与上方本系统状态同名的词
// （paid/pending/failed/refunded）直接复用 status* 常量，不另立同值常量。
const (
	gatewayCompletedUpper = "COMPLETED"
	gatewayCancelledUpper = "CANCELLED"
	gatewayRefundedUpper  = "REFUNDED"
	gatewaySuccessUpper   = "SUCCESS"
	gatewaySucceededUpper = "SUCCEEDED"

	gatewayCompletedLower = "completed"
	gatewayCanceledLower  = "canceled"
	gatewaySucceededLower = "succeeded"
)

// 渠道标识：工厂按渠道名分发网关，回调路由也按同一名字分流。
const (
	channelAlipay       = "alipay"
	channelWeChatPay    = "wechat_pay"
	channelStripe       = "stripe"
	channelPayPal       = "paypal"
	channelAdyen        = "adyen"
	channelAirwallex    = "airwallex"
	channelSquare       = "square"
	channelCheckout     = "checkout"
	channelPaddle       = "paddle"
	channelFastSpring   = "fastspring"
	channelLemonSqueezy = "lemonsqueezy"
	channelPolar        = "polar"
	channelBalance      = "balance"
	channelGC           = "gc"
	channelDummy        = "dummy"
)
