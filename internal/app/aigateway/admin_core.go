// admin_core.go — AdminService 核心结构、构造函数与共享工具方法。
//
// 放在 App 层的唯一原因是 transport 层被架构规约（Rule 2）禁止
// 直接 import domain 包；DTO↔Domain 转换函数因此必须落在此处。
// AdminService 持有 domain 层定义的 Repo 接口、平台层 uuid/clock
// 以及一个可选的 FieldEncryptor（Provider API key 加密）。
package aigateway

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// AdminService 是管理面的应用服务。它只操作 domain 层 Repo 接口，
// 不关心底层持久化是 PG 还是 MySQL。
type AdminService struct {
	UUID      platformuuid.Generator
	Clock     clock.Clock
	Encryptor *crypto.FieldEncryptor
	Providers domain.ProvidersRepo
	Models    domain.ModelsRepo
	Aliases   domain.AliasesRepo
	Routes    domain.RoutesRepo
	ApiKeys   domain.ApiKeysRepo
	Quotas    domain.QuotaPoliciesRepo
	UsageLogs domain.UsageLogsRepo
	Prices    domain.PriceVersionsRepo
	Health    domain.ProviderHealthRepo
	// Upstreams 是 TestProvider 复用的统一适配器构建端口。bootstrap 注入
	// aiupstream.AdapterFactory（含 provider 凭据解密与协议归一化）；nil 时
	// TestProvider 返回明确的 unavailable 错误，而不是退化为无凭据探测。
	Upstreams domain.ProviderRegistry
}

// NewAdminService 构造一个 AdminService。enc 可为 nil —— 当调用方不传时，
// Provider.Create 会退化为 GlobalFieldEncryptor()（开发/CI 方便）。
func NewAdminService(
	u platformuuid.Generator,
	c clock.Clock,
	enc *crypto.FieldEncryptor,
	providers domain.ProvidersRepo,
	models domain.ModelsRepo,
	aliases domain.AliasesRepo,
	routes domain.RoutesRepo,
	apiKeys domain.ApiKeysRepo,
	quotas domain.QuotaPoliciesRepo,
	usageLogs domain.UsageLogsRepo,
	prices domain.PriceVersionsRepo,
	health domain.ProviderHealthRepo,
) *AdminService {
	return &AdminService{
		UUID:      u,
		Clock:     c,
		Encryptor: enc,
		Providers: providers,
		Models:    models,
		Aliases:   aliases,
		Routes:    routes,
		ApiKeys:   apiKeys,
		Quotas:    quotas,
		UsageLogs: usageLogs,
		Prices:    prices,
		Health:    health,
	}
}

func (s *AdminService) encryptAPIKey(plaintext string) (string, error) {
	if strings.TrimSpace(plaintext) == "" {
		return "", apperrors.NewInvalidArgument("apiKey is required")
	}
	enc := s.Encryptor
	if enc == nil {
		enc = crypto.GlobalFieldEncryptor()
	}
	return enc.EncryptField(plaintext)
}

// fingerprintAPIKey 生成一个展示用的指纹 —— "sk-xxxx...xxxx"，
// 比 GenerateKey 里的 prefix 更短、更通用。
func fingerprintAPIKey(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	// 用 SHA-256 的前 4 字节 hex 作为稳定指纹
	sum := sha256.Sum256([]byte(plaintext))
	return "sk-****-" + hex.EncodeToString(sum[:4])
}
