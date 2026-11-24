package payments

// 渠道状态词。不同渠道恰好复用同一字面量（微信/支付宝/Stripe/PayPal 等），
// 大小写是协议的一部分；表仍按渠道分组，命名只为消除重复字面量，值逐字保留。
const (
	channelStatusSuccess   = "SUCCESS"
	channelStatusSucceeded = "SUCCEEDED"
	channelStatusPending   = "PENDING"
	channelStatusFailed    = "FAILED"
	channelStatusCancelled = "CANCELLED"
)
