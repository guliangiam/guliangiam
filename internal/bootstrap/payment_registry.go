package bootstrap

import (
	"log/slog"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	infrapayment "github.com/guliangiam/guliangiam/internal/infra/payment"
)

// defaultGateways 列出工厂支持的全部渠道，取自 infra 的唯一渠道声明。
// 每个渠道的 credentials 从环境变量读取（工厂内部处理）。
var defaultGateways = infrapayment.ChannelNames()

// InitGatewayRegistry 在 bootstrap 启动时创建 GatewayRegistry 并用
// 每个渠道的 ChannelAdapter（ACL 包装 PaymentGateway → ChannelGateway SPI）注册。
// 返回的 registry 注入到 CallbackPipeline，让回调管线能按 channel 找到
// adapter 做 ParseCallback / Charge / Query / Refund。
func InitGatewayRegistry() *apppayments.GatewayRegistry {
	reg := apppayments.NewGatewayRegistry()
	for _, name := range defaultGateways {
		// 构造占位 config — 真实 credentials 在 RuntimeConfig 层按渠道注入。
		// NewGateway 对部分渠道（如 dummy/balance/fastspring）不需要 config。
		// WeChat Pay 需要 APIv3 的商户私钥、证书序列号与平台证书 bundle。
		config := map[string]string{
			paymentConfigAPIKey:    envOrEmpty("PAYMENT_API_KEY"),
			"clientId":             envOrEmpty("PAYMENT_CLIENT_ID"),
			"clientSecret":         envOrEmpty("PAYMENT_CLIENT_SECRET"),
			"appId":                envOrEmpty("PAYMENT_APP_ID"),
			"mchId":                envOrEmpty("PAYMENT_MCH_ID"),
			"serialNo":             envOrEmpty("PAYMENT_MCH_SERIAL_NO"),
			"privateKey":           envOrEmpty("PAYMENT_PRIVATE_KEY"),
			"platformCertPEM":      envOrEmpty("PAYMENT_PLATFORM_CERT_PEM"),
			paymentConfigIsSandbox: envOrEmpty("PAYMENT_IS_SANDBOX"),
		}
		gw, err := infrapayment.NewGateway(name, config)
		if err != nil {
			// 缺少必要 credentials 的渠道跳过 — 单渠道失败不应阻止其他渠道注册。
			// 日志区分「未配置」与「构造失败」，否则 wxpay 之类会静默消失。
			slog.Warn("payment channel not registered", "channel", name, "error", err)
			continue
		}
		reg.Register(name, infrapayment.NewChannelAdapter(name, gw))
	}
	return reg
}
