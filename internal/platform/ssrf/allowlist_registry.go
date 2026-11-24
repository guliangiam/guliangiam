package ssrf

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AllowlistEntry 描述一个被 SSRF 防护系统**显式放行**的业务场景。
//
// 集中管理目的：
//  1. 审计可见 —— 一眼看到「谁、为什么、放哪些网段」
//  2. 与默认阻断行为形成对照（IsSafeIP 默认断私有网段，这里显式放行是例外）
//  3. 后续可加环境变量强制禁用某条（CSP 模式）
type AllowlistEntry struct {
	// Name 业务名（唯一标识符，用于日志 + 审计）
	Name string
	// Description 业务理由（必须解释为什么这个业务需要放行）
	Description string
	// CIDRs 放行的网段（"10.0.0.0/8" 等）
	CIDRs []string
}

// allowlistRegistry 集中注册所有 SSRF 放行场景。
// 任何业务如果需要放行私有网段，**必须**在此处注册，而不是在业务代码里硬编码。
//
// 审查红线：每个 Entry 必须回答两个问题 ——
//
//	a) 为什么这个业务需要访问私有网段（业务合法性）？
//	b) 放哪些网段（最小原则）？
//
// 例：内网扫描是合法业务，但只放 RFC1918 / loopback / ULA，绝对不放 metadata 169.254.0.0/16。
var allowlistRegistry = []AllowlistEntry{
	{
		Name:        "mcpservers.intranet_scan",
		Description: "合法内网资产发现：扫 RFC1918 + loopback + ULA，不放 metadata 段（169.254.0.0/16 始终阻断）",
		CIDRs: []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"127.0.0.0/8",
			"::1/128",
			"fc00::/7", // ULA
		},
	},
}

// CIDRsFor 返回 named entry 的 CIDR 列表。未注册返回 nil（零值 = 默认严格阻断）。
//
// 业务代码调用示例：
//
//	client := ssrf.SafeHTTPClientWithAllowlist(5*time.Second, ssrf.CIDRsFor("mcpservers.intranet_scan"))
func CIDRsFor(name string) []string {
	for _, e := range allowlistRegistry {
		if e.Name == name {
			// 返回副本，防止业务代码修改注册表
			out := make([]string, len(e.CIDRs))
			copy(out, e.CIDRs)
			return out
		}
	}
	return nil
}

// ListAllowlistEntries 返回当前所有注册的 allowlist 条目（用于健康检查 / 运维诊断）。
func ListAllowlistEntries() []AllowlistEntry {
	out := make([]AllowlistEntry, len(allowlistRegistry))
	copy(out, allowlistRegistry)
	return out
}

// HasEntry 返回指定业务是否有 SSRF allowlist 注册。
func HasEntry(name string) bool {
	for _, e := range allowlistRegistry {
		if e.Name == name {
			return true
		}
	}
	return false
}

// NamedHTTPClient 返回一个带 SSRF 防护 + 指定业务 allowlist 的 *http.Client。
// name 未注册时 allowlist 为 nil（严格阻断模式）。
func NamedHTTPClient(name string, timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return SafeHTTPClientWithAllowlist(timeout, CIDRsFor(name))
}

// NamedHTTPClientOrDie 返回带 allowlist 的 client；name 未注册时 panic。
// 用于启动期检查（如果业务声明需要 allowlist 却没注册，进程应在启动时就失败）。
func NamedHTTPClientOrDie(name string, timeout time.Duration) *http.Client {
	if !HasEntry(name) {
		panic("ssrf: allowlist entry not registered for " + name +
			" — add to allowlistRegistry or use SafeHTTPClient")
	}
	return NamedHTTPClient(name, timeout)
}

// ValidateRegistry 在启动时调用，拒绝非法 CIDR（防止 typo 导致放行/阻断错位）。
func ValidateRegistry() error {
	for _, e := range allowlistRegistry {
		if strings.TrimSpace(e.Name) == "" {
			return apperrors.NewInvalidArgument("allowlist: entry has empty Name")
		}
		if strings.TrimSpace(e.Description) == "" {
			return apperrors.NewInvalidArgument(fmt.Sprintf("allowlist: entry %q has empty Description", e.Name))
		}
		if len(e.CIDRs) == 0 {
			return apperrors.NewInvalidArgument(fmt.Sprintf("allowlist: entry %q has empty CIDRs", e.Name))
		}
		// metadata 段禁止出现在任何 allowlist
		for _, c := range e.CIDRs {
			if c == "169.254.0.0/16" || strings.HasPrefix(c, "169.254.") {
				return apperrors.NewInvalidArgument(fmt.Sprintf("allowlist: entry %q must not include metadata CIDR 169.254.0.0/16", e.Name))
			}
		}
	}
	return nil
}
