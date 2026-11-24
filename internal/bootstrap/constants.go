package bootstrap

// 存储与支付配置里重复出现的字面量。
const (
	defaultUploadsDir = "./uploads"

	// 支付网关 config map 的键：由 internal/infra/payment.NewGateway 按
	// "apiKey" / "isSandbox" 读取，值必须与其保持一致。
	paymentConfigAPIKey    = "apiKey"
	paymentConfigIsSandbox = "isSandbox"
)

// DB_DRIVER 取值与 dialect registry key。驱动名与方言键刻意共用同一组字面量：
// driverNormalize 与 wire_db 的 dialectKey 映射在同一个启动流程里往返转换，
// 值必须与 internal/infra/dialect 的注册键逐字一致。
const (
	driverPgx       = "pgx"
	driverPostgres  = "postgres"
	driverMySQL     = "mysql"
	driverKingbase  = "kingbase"
	driverGaussDB   = "gaussdb"
	driverOceanBase = "oceanbase"
	driverTiDB      = "tidb"
	driverPolarDB   = "polardb"
)

// 字符串编码的布尔值：环境变量、启动 flag 与 config getter 统一以 "true" 表示开启。
const valueTrue = "true"
