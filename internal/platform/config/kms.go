// ============================================================================
// KMS Abstraction — ADR-009 Target State 落地
// ============================================================================
// GuliangIAM 当前以环境变量提供根密钥；生产态（ADR-009）要求 KMS/Vault 托管。
// 本文件定义 KeyProvider 端口，让 env.go 的密钥解析函数变成 provider 的薄包装，
// 生产环境可通过 KMS_PROVIDER=vault 切换到 Vault adapter。
//
// Provider 选择时机：
//   1. bootstrap 启动时、ValidateOrPanic 之前调
//      config.InitKMSProvider(os.Getenv("KMS_PROVIDER"))，使密钥存在性校验
//      走已选 provider（Vault 托管密钥不会被误判为环境变量缺失）
//   2. 默认 EnvKeyProvider（当前实现，向后兼容）
//   3. VaultKeyProvider stub 预留 HashiCorp Vault KV v2 读取
// ============================================================================

package config

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// KeyProvider 是根密钥的抽象来源。实现方可以是环境变量（过渡态）、
// HashiCorp Vault、云 KMS 或本地密钥文件。所有实现必须是并发安全的——
// InitKMSProvider 在 bootstrap 中只调用一次，之后只读。
type KeyProvider interface {
	// EncryptionKeyCurrent 返回当前活跃字段加密密钥（KEK v1）。
	EncryptionKeyCurrent() string
	// EncryptionKeyLegacy 返回 legacy 密钥（ENCRYPTION_KEY），用于过渡期解密。
	EncryptionKeyLegacy() string
	// EncryptionKeyPrevious 返回上一版密钥（ENCRYPTION_KEY_PREVIOUS），仅解密回退用。
	EncryptionKeyPrevious() string
	// EncryptionKeyCompat 返回算法切换重叠窗口内另一算法的解密密钥（ENCRYPTION_KEY_COMPAT）。
	EncryptionKeyCompat() string
	// ETagHMACKey 返回 ETag middleware HMAC key（ETAG_HMAC_KEY）。
	ETagHMACKey() string
	// CursorHMACKey 返回 cursor pagination HMAC key（CURSOR_HMAC_KEY）。
	CursorHMACKey() string
	// PasswordHistoryPepper 返回密码历史 pepper（PASSWORD_HISTORY_PEPPER）。
	PasswordHistoryPepper() string
	// Describe 用于日志：provider 类型名 + 来源描述（不打印密钥内容）。
	Describe() string
}

// ----------------------------------------------------------------------------
// 包级 provider（InitKMSProvider 在 bootstrap 中注入，默认 EnvKeyProvider）
// ----------------------------------------------------------------------------

var currentKeyProvider KeyProvider = &envKeyProvider{}

// InitKMSProvider 根据 KMS_PROVIDER 环境变量选择 provider。
// 调用时机：bootstrap 在 ValidateOrPanic 之前调用一次，随后字段加密器构造
// 读取 provider 提供的密钥。之后 provider 只读。
func InitKMSProvider(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "env", "environment":
		currentKeyProvider = &envKeyProvider{}
	case "vault":
		currentKeyProvider = newVaultKeyProviderFromEnv()
	case "none", "disabled":
		slog.Warn("KMS_PROVIDER=none — 使用 EnvKeyProvider（仅开发/测试）")
		currentKeyProvider = &envKeyProvider{}
	default:
		slog.Warn("未知 KMS_PROVIDER，回退 EnvKeyProvider", "provider", name)
		currentKeyProvider = &envKeyProvider{}
	}
	slog.Info("KMS provider initialized", "provider", currentKeyProvider.Describe())
	return currentKeyProvider.Describe()
}

// ----------------------------------------------------------------------------
// EnvKeyProvider — 当前实现（环境变量）
// ----------------------------------------------------------------------------

type envKeyProvider struct{}

func (e *envKeyProvider) EncryptionKeyCurrent() string {
	return strings.TrimSpace(os.Getenv("ENCRYPTION_KEY_CURRENT"))
}
func (e *envKeyProvider) EncryptionKeyLegacy() string {
	return strings.TrimSpace(os.Getenv("ENCRYPTION_KEY"))
}
func (e *envKeyProvider) EncryptionKeyPrevious() string {
	return strings.TrimSpace(os.Getenv("ENCRYPTION_KEY_PREVIOUS"))
}
func (e *envKeyProvider) EncryptionKeyCompat() string {
	return strings.TrimSpace(os.Getenv("ENCRYPTION_KEY_COMPAT"))
}

func (e *envKeyProvider) ETagHMACKey() string   { return os.Getenv("ETAG_HMAC_KEY") }
func (e *envKeyProvider) CursorHMACKey() string { return os.Getenv("CURSOR_HMAC_KEY") }
func (e *envKeyProvider) PasswordHistoryPepper() string {
	return os.Getenv("PASSWORD_HISTORY_PEPPER")
}

func (e *envKeyProvider) Describe() string { return "env (environment variables)" }

// ----------------------------------------------------------------------------
// VaultKeyProvider stub — HashiCorp Vault KV v2 读取
// ----------------------------------------------------------------------------
// 读取路径约定：
//   secrets/data/guliangiam/production/keys
//   键名: session_hmac_key, encryption_key_current, encryption_key_legacy, ...
//
// 生产接入步骤（A3）：
//   1. 部署 HashiCorp Vault 或云 KMS
//   2. 设置 KMS_PROVIDER=vault VAULT_ADDR VAULT_TOKEN
//   3. 在 Vault KV v2 secrets/data/guliangiam/production 写入密钥
//   4. 运维演练：密钥轮换流程 + 双人审批 + 销毁
//
// 当前 stub 用 http.Client + KV v2 API，不用 Vault SDK。
// ----------------------------------------------------------------------------

type vaultKeyProvider struct {
	addr    string
	token   string
	path    string // Vault KV v2 mount path，默认 "secrets/data/guliangiam/production/keys"
	client  *http.Client
	cached  map[string]string
	readErr error // 首次 ReadAll 失败时记录，后续各方法返回空串（让 validate.go 触发 fail-fast）
}

func newVaultKeyProviderFromEnv() *vaultKeyProvider {
	p := &vaultKeyProvider{
		addr:  strings.TrimRight(os.Getenv("VAULT_ADDR"), "/"),
		token: os.Getenv("VAULT_TOKEN"),
		path:  firstNonEmpty(os.Getenv("VAULT_KEYS_PATH"), "secrets/data/guliangiam/production/keys"),
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13},
			},
		},
	}
	p.ReadAll()
	return p
}

// ReadAll 向 Vault KV v2 拉取一次全量键值对，缓存到 p.cached。
// 首次启动调一次即可——密钥轮换由 Vault 侧 + 应用重启完成（ADR-015 密钥版本化）。
func (p *vaultKeyProvider) ReadAll() {
	p.cached = make(map[string]string)
	if p.addr == "" || p.token == "" {
		p.readErr = apperrors.NewInternalf("VAULT_ADDR / VAULT_TOKEN 未设置")
		slog.Warn("Vault 未配置，密钥解析将返回空串（validate.go 会触发 fail-fast）")
		return
	}

	url := fmt.Sprintf("%s/v1/%s", p.addr, p.path)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		p.readErr = err
		return
	}
	req.Header.Set("X-Vault-Token", p.token)

	resp, err := p.client.Do(req)
	if err != nil {
		p.readErr = err
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		p.readErr = apperrors.NewInternalf("Vault HTTP %d: %s", resp.StatusCode, string(b))
		return
	}

	// KV v2 response 结构：{"data": {"data": {...}}}
	buf, _ := io.ReadAll(resp.Body)
	var outer struct {
		Data struct {
			Data map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf, &outer); err != nil {
		p.readErr = fmt.Errorf("vault 响应 JSON 解析失败: %w", err)
		return
	}
	for k, v := range outer.Data.Data {
		switch val := v.(type) {
		case string:
			p.cached[k] = val
		case float64:
			// Vault 数值类型存为 JSON number → string 回退
			p.cached[k] = fmt.Sprintf("%v", val)
		default:
			p.cached[k] = fmt.Sprintf("%v", val)
		}
	}
	if len(p.cached) == 0 {
		p.readErr = apperrors.NewInternalf("Vault 路径 %s 无有效键值对", p.path)
	}
}

func (p *vaultKeyProvider) Describe() string {
	if p.readErr != nil {
		return fmt.Sprintf("vault (stub, error: %v)", p.readErr)
	}
	return fmt.Sprintf("vault (%s/%s)", p.addr, p.path)
}

func (p *vaultKeyProvider) get(key string) string {
	if p.readErr != nil {
		return ""
	}
	if v, ok := p.cached[key]; ok {
		return v
	}
	return ""
}

// ---- 以下各方法委托到 p.get（stub 实现，等 ReadAll 的 JSON 解析完善后生效） ----

func (p *vaultKeyProvider) EncryptionKeyCurrent() string  { return p.get("encryption_key_current") }
func (p *vaultKeyProvider) EncryptionKeyLegacy() string   { return p.get("encryption_key_legacy") }
func (p *vaultKeyProvider) EncryptionKeyPrevious() string { return p.get("encryption_key_previous") }
func (p *vaultKeyProvider) EncryptionKeyCompat() string   { return p.get("encryption_key_compat") }
func (p *vaultKeyProvider) ETagHMACKey() string           { return p.get("etag_hmac_key") }
func (p *vaultKeyProvider) CursorHMACKey() string         { return p.get("cursor_hmac_key") }
func (p *vaultKeyProvider) PasswordHistoryPepper() string { return p.get("password_history_pepper") }

func firstNonEmpty(candidates ...string) string {
	for _, s := range candidates {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
