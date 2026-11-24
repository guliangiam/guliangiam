// Package ratelimit 实现 HTTP 限流响应头的**唯一**写入点。
//
// 为什么必须集中：限流头是客户端退避、网关归因与 SLO 计算的输入。此前多处中间件
// 各自手写已废弃的 `X-` 前缀限流头（naming-registry N11），没有一个 IETF 字段，
// `Retry-After` 还出现过 Go 时长字符串（`"1s"`）——`Retry-After` 的语法只有
// delta-seconds 整数或 HTTP-date，客户端解析失败后只能退化为固定间隔重试，
// 等于限流形同不生效。
//
// 口径来源：《GuliangIAM 开发规范手册》4.4.1，唯一字段清单与维度定义见
// docs/registries/limit-registry.md。本包只负责**呈现**，配额数值仍由各中间件配置提供。
package ratelimit

import (
	"math"
	"net/http"
	"strconv"
	"time"
)

// 响应头字段名。集中声明，杜绝调用点拼写漂移。
const (
	HeaderLimit      = "RateLimit-Limit"
	HeaderRemaining  = "RateLimit-Remaining"
	HeaderReset      = "RateLimit-Reset"
	HeaderPolicy     = "RateLimit-Policy"
	HeaderRetryAfter = "Retry-After"
)

// Dimension 描述一个限流维度（《开发规范手册》4.4.1 要求至少覆盖 IP / 租户 / 用户 /
// client / 端点族五层，各层独立计数并以策略 id 区分）。
type Dimension struct {
	// ID 是策略标识，写入 RateLimit-Policy 的 comment 参数，用于跨层归因。
	// 例如 "global"、"auth"、"service-ip:/api/v1/auth/login"。
	ID string
	// Quota 是窗口内的请求配额；Burst 是令牌桶容量（为 0 表示等于 Quota）。
	Quota int64
	Burst int64
	// Window 是配额窗口长度，写入 RateLimit-Policy 的 w 参数（整数秒）。
	Window time.Duration
}

// Snapshot 是一次请求结束时某维度对客户端可见的状态。
type Snapshot struct {
	Dimension
	// Remaining 是当前窗口剩余额度（向下取整，非负）。
	Remaining int64
	// Reset 是距额度恢复的相对时长，按向上取整折算为秒后写入 RateLimit-Reset。
	Reset time.Duration
}

// Policy 返回 `RateLimit-Policy` 的值，形如 `100;w=1;burst=200;comment="global"`。
// 窗口不足 1 秒时按 1 秒向上取整：`w` 只接受整数秒，写 0 会被客户端当作无限窗口。
func (d Dimension) Policy() string {
	seconds := int64(math.Ceil(d.Window.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	out := strconv.FormatInt(d.Quota, 10) + ";w=" + strconv.FormatInt(seconds, 10)
	if d.Burst > 0 && d.Burst != d.Quota {
		out += ";burst=" + strconv.FormatInt(d.Burst, 10)
	}
	if d.ID != "" {
		out += `;comment="` + d.ID + `"`
	}
	return out
}

// Write 写出一个维度的全部限流反馈字段。成功响应与被拒响应都应调用：规范要求
// 「成功与被限响应均应携带」限流头；只在 429 时给出额度，客户端就无法在做满之前
// 主动降速。
func Write(h http.Header, s Snapshot) {
	limit := s.Quota
	if limit < s.Burst {
		// 令牌桶允许瞬时突刺；对客户端可见的上限必须是桶容量，否则 Limit < Remaining。
		limit = s.Burst
	}
	remaining := s.Remaining
	if remaining < 0 {
		remaining = 0
	}
	if remaining > limit {
		remaining = limit
	}
	h.Set(HeaderLimit, strconv.FormatInt(limit, 10))
	h.Set(HeaderRemaining, strconv.FormatInt(remaining, 10))
	h.Set(HeaderReset, strconv.Itoa(ResetSeconds(s.Reset)))
	h.Set(HeaderPolicy, s.Policy())
}

// WriteRetryAfter 在拒绝响应上写出重试指引。**只接受整数秒**：`Retry-After` 的两种
// 合法形式是 delta-seconds 与 HTTP-date，Go 时长字符串（如 `1s`）两者都不是。
// 不足 1 秒按 1 秒计，写 0 会让客户端立即重试并放大压力。
func WriteRetryAfter(h http.Header, after time.Duration) {
	h.Set(HeaderRetryAfter, strconv.Itoa(ResetSeconds(after)))
}

// ResetSeconds 把相对时长按向上取整折算为非负整数秒。
func ResetSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	s := int(math.Ceil(d.Seconds()))
	if s < 1 {
		s = 1
	}
	return s
}

// Consume 从令牌桶状态推导快照：tokens 为 Allow 之后调用 limiter.Tokens() 得到的
// 当前令牌数（即已扣减本次请求），ratePerSecond 为每秒补充速率。
//
// Reset 取「恢复一个令牌所需时间」，也就是客户端最早可以再试的时刻；令牌桶没有固定
// 窗口边界，报告一个假的窗口边界会误导客户端的退避策略。
func Consume(d Dimension, tokens, ratePerSecond float64) Snapshot {
	remaining := int64(math.Floor(tokens))
	if remaining < 0 {
		remaining = 0
	}
	var reset time.Duration
	switch {
	case remaining > 0:
		reset = 0
	case ratePerSecond > 0:
		reset = time.Duration(math.Ceil(1000/ratePerSecond)) * time.Millisecond
	default:
		// 速率为 0 表示该维度实际不恢复；报 0 会让客户端死循环重试，
		// 故退化为窗口长度，至少给出一个有界的退避上界。
		reset = d.Window
	}
	return Snapshot{Dimension: d, Remaining: remaining, Reset: reset}
}
