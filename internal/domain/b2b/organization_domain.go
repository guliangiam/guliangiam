package b2b

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// OrganizationDomain 聚合根 —— B2B 企业域名认领 + DNS TXT 验证。
//
// functional.md §15.1 端到端：
//
//	post /organizations/{uuid}/domains → 生成一次性 TXT token
//	→ DNS 管理员在 _guliangiam-verification.<domain> 添加 TXT 记录
//	→ Cron 每 2min 对 pending 执行 DNS 查询
//	→ 匹配成功 → pending→verified → 自动启用邮箱域 SSO 发现
//
// 状态机（DDL ck_organization_domains_status 同一套值）：
//
//	pending ──verify()──▶ verified
//	   │
//	   ├──retry()─────▶ pending（记录 attempts / last_error / next_retry_at）
//	   │
//	   └──fail()──────▶ failed（重试预算耗尽）
//
//	verified 和 failed 都是终态；只能重新认领（新行）。重试期间仍保持 pending，
//	因此 DDL 的三态集合不变。
type OrganizationDomain struct {
	ID                string    `json:"id"`
	OrganizationID    string    `json:"organizationId"`
	Domain            string    `json:"domain"`
	VerificationToken string    `json:"verificationToken"`
	Status            string    `json:"status"`
	VerifiedAt        time.Time `json:"verifiedAt,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`

	// 续验预算与调度字段。不通过 HTTP DTO 暴露（B7-g 边界）。
	Attempts      int       `json:"-"` // 已失败的续验次数
	LastError     string    `json:"-"` // 脱敏后的分类码 + 简短说明；空 = 无
	NextRetryAt   time.Time `json:"-"` // 下一次允许认领的时间；零值 = 立即可认领
	LastCheckedAt time.Time `json:"-"` // 最近一次实际 DNS 查询时间；零值 = 从未查询
}

// MaxLastErrorRunes 是 last_error VARCHAR(255) 的字符上限（PostgreSQL 按字符计数）。
const MaxLastErrorRunes = 255

// 状态常量 —— §15.1 状态机 closed set。
const (
	StatusPending  = "pending"
	StatusVerified = "verified"
	StatusFailed   = "failed"
)

// Statuses 是有序封闭集。DDL ck_organization_domains_status 与 API 枚举从这一份生成。
var Statuses = enumset.NewSet("organizationDomainStatus",
	StatusPending, StatusVerified, StatusFailed)

// terminalStatuses 终态集合 —— verified 和 failed 不可逆。
var terminalStatuses = map[string]bool{
	StatusVerified: true,
	StatusFailed:   true,
}

// IsTerminal 判断当前状态是否是终态。
func IsTerminal(status string) bool {
	return terminalStatuses[status]
}

// NormalizeDomain 按 DNS 的大小写不敏感语义统一认领与发现输入。
func NormalizeDomain(name string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), "."))
}

// Validate 业务规则：
//  1. Status 必须在封闭集内
//  2. Domain 必须是合法域名（net.ParseCIDR / net.LookupHost 不可用，用 net.ParseIP 反向排除 IP 输入）
//  3. VerificationToken 非空
func (d *OrganizationDomain) Validate() error {
	d.Domain = NormalizeDomain(d.Domain)
	if d.Status == "" {
		d.Status = StatusPending
	}
	if !Statuses.Contains(d.Status) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid organization_domain status: %s", d.Status),
			map[string]any{"field": "status", "value": d.Status, "allowed": Statuses.SortedValues()},
		)
	}
	if d.Domain == "" {
		return apperrors.NewInvalidArgument("domain is required")
	}
	if isIP(d.Domain) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "domain must be a DNS name, not an IP address", map[string]any{"field": "domain", "value": d.Domain, "format": "dns_name"})
	}
	if !isValidDomainFormat(d.Domain) {
		return apperrors.NewInvalidArgumentf("domain %q is not a valid DNS name", d.Domain)
	}
	if d.VerificationToken == "" {
		return apperrors.NewInvalidArgument("verification_token is required")
	}
	return nil
}

// MarkVerified 状态迁移 pending→verified，设置 VerifiedAt。
// 终态不可逆：如果已经在 verified / failed，返回 InvalidArgument。
func (d *OrganizationDomain) MarkVerified(now time.Time) error {
	if d.Status != StatusPending {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot verify domain in state %s (only pending is allowed)", d.Status),
			map[string]any{"field": "status", "value": d.Status, "expected": StatusPending},
		)
	}
	d.Status = StatusVerified
	d.VerifiedAt = now
	d.UpdatedAt = now
	return nil
}

// MarkFailed 状态迁移 pending→failed。终态不可逆。
func (d *OrganizationDomain) MarkFailed(now time.Time) error {
	if d.Status != StatusPending {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot mark domain as failed in state %s", d.Status),
			map[string]any{"field": "status", "value": d.Status, "expected": StatusPending},
		)
	}
	d.Status = StatusFailed
	d.UpdatedAt = now
	return nil
}

// RecordVerificationFailure 记录一次失败的 DNS 续验并推进重试预算。
//
//   - attempts++、last_error = 截断后的 reason、last_checked_at = now；
//   - attempts >= maxAttempts → 走 MarkFailed（failed 终态，next_retry_at 不再参与认领）；
//   - 否则保持 pending，next_retry_at = now + backoff(attempts)。
//
// maxAttempts < 1 视为 1（一次失败即终态）；backoff 为 nil 或返回负值时按 0 处理。
func (d *OrganizationDomain) RecordVerificationFailure(
	reason string, now time.Time, maxAttempts int, backoff func(attempts int) time.Duration,
) error {
	if d.Status != StatusPending {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot record verification failure in state %s", d.Status),
			map[string]any{"field": "status", "value": d.Status, "expected": StatusPending},
		)
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	d.Attempts++
	d.LastError = truncateLastError(reason)
	d.LastCheckedAt = now
	d.UpdatedAt = now

	if d.Attempts >= maxAttempts {
		return d.MarkFailed(now)
	}

	if backoff == nil {
		d.NextRetryAt = now
		return nil
	}
	delay := backoff(d.Attempts)
	if delay < 0 {
		delay = 0
	}
	d.NextRetryAt = now.Add(delay)
	return nil
}

// RecordVerificationSuccess 记录一次成功的 DNS 续验：走既有 MarkVerified，
// 并清空 last_error、写入最近查询时间。
func (d *OrganizationDomain) RecordVerificationSuccess(now time.Time) error {
	if err := d.MarkVerified(now); err != nil {
		return err
	}
	d.LastError = ""
	d.LastCheckedAt = now
	return nil
}

// IsDue 判断当前行是否已到可认领时间：仅 pending 且 next_retry_at 不晚于 now。
// NextRetryAt 零值视为立即可认领（兼容未从数据库回读的构造对象）。
func (d *OrganizationDomain) IsDue(now time.Time) bool {
	if d.Status != StatusPending {
		return false
	}
	if d.NextRetryAt.IsZero() {
		return true
	}
	return !now.Before(d.NextRetryAt)
}

// truncateLastError 按字符（而非字节）截断，避免切断 UTF-8 码点。
func truncateLastError(s string) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= MaxLastErrorRunes {
		return s
	}
	return string(runes[:MaxLastErrorRunes])
}

// isIP 反向排除 IP 地址输入（net.ParseIP 返回非 nil 即视为 IP）。
func isIP(s string) bool {
	return net.ParseIP(s) != nil
}

// isValidDomainFormat 做轻量 DNS 名称验证：
//   - 非空 + 不超过 253 字符
//   - 每个 label 1-63 字符，字母数字连字符（首尾不能是连字符）
//   - 至少有一个 dot（二级以上域名）
//
// 这是与 net.LookupHost / net.Resolver 解耦的纯本地校验——
// Create 时就可以拒绝明显非法输入，避免 DNS 空查消耗。
func isValidDomainFormat(name string) bool {
	if name == "" || len(name) > 253 {
		return false
	}
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	if !containsDot(name) {
		return false
	}
	for _, label := range splitDot(name) {
		if label == "" || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if !isLetterOrDigitOrHyphen(ch) {
				return false
			}
		}
	}
	return true
}

func containsDot(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return true
		}
	}
	return false
}

func splitDot(s string) []string {
	out := make([]string, 0, 8)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// isLetterOrDigitOrHyphen 只接受 ASCII 字母/数字/连字符；非 ASCII rune 因不落在
// 这些区间而被拒绝，避免先转 byte 时把码点截断成合法字符。
func isLetterOrDigitOrHyphen(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-'
}
