package webauthn

import (
	"net/url"
	"strings"
)

// RP ID（Relying Party ID）是 WebAuthn 的作用域：断言只在该作用域内有效，且必须等于
// 来源主机或其可注册后缀——「租户 slug 当 RP ID」这类值（如 acme）不是合法域，浏览器会
// 直接拒绝 ceremony（《开发设计》31.2 缺陷表登记）。
//
// 这里从服务端配置的来源集合推导：取所有来源主机的**公共后缀**（按标签边界比较），
// 因此同源部署得到主机本身（http://localhost:7101 → localhost），
// 控制台与 API 分处不同子域时得到可注册域（console.example.com + iam.example.com →
// example.com），两种情况都满足「是每个来源主机的后缀」这条硬约束。

// RelyingPartyIDFor 从允许来源清单推导 RP ID。
// 空清单、跨可注册域的混合来源、以及除 localhost 外的单标签主机（如内部域名 svc）都返回错误——
// 这些值浏览器不会接受，宁可 fail-closed 也不要让用户看到一个无法完成的弹窗。
func RelyingPartyIDFor(origins []string) (string, error) {
	hosts := make([]string, 0, len(origins))
	for _, origin := range origins {
		normalized := NormalizeOrigin(origin)
		if normalized == "" {
			// 通配符条目或不可解析的条目：不能参与作用域推导（语义也不确定）。
			continue
		}
		parsed, err := url.Parse(normalized)
		if err != nil || parsed.Host == "" {
			continue
		}
		hosts = append(hosts, strings.ToLower(parsed.Hostname()))
	}
	if len(hosts) == 0 {
		return "", errRelyingPartyIDUnavailable("no usable origin in the WebAuthn origin allowlist")
	}

	common := hosts[0]
	for _, host := range hosts[1:] {
		common = commonLabelSuffix(common, host)
		if common == "" {
			return "", errRelyingPartyIDUnavailable("origins span different registrable domains, so no single RP ID covers them all")
		}
	}

	if !strings.Contains(common, ".") && common != "localhost" {
		// 单标签主机只有 localhost 是浏览器接受的 RP ID；其余（如 svc、acme）会被拒绝。
		return "", errRelyingPartyIDUnavailable("derived RP ID " + common + " is a single label; only localhost is accepted in that shape")
	}
	return common, nil
}

// commonLabelSuffix 返回两个主机按标签对齐的公共后缀；无公共后缀（顶层标签就不同）时返回空串。
// 按标签比较是必须的：字符串后缀会把 notexample.com 误判成 example.com 的后缀。
func commonLabelSuffix(a, b string) string {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	common := make([]string, 0, len(aParts))
	for i := 1; i <= len(aParts) && i <= len(bParts); i++ {
		left := aParts[len(aParts)-i]
		right := bParts[len(bParts)-i]
		if left != right {
			break
		}
		common = append([]string{left}, common...)
	}
	return strings.Join(common, ".")
}

type relyingPartyIDError struct{ reason string }

func (e *relyingPartyIDError) Error() string { return e.reason }

func errRelyingPartyIDUnavailable(reason string) error { return &relyingPartyIDError{reason: reason} }
