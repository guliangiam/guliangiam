package ratelimit

import (
	"net/http"
	"testing"
	"time"
)

func TestDimensionPolicy(t *testing.T) {
	cases := []struct {
		name string
		dim  Dimension
		want string
	}{
		{
			name: "配额与桶容量相同则省略 burst",
			dim:  Dimension{ID: "auth-ip", Quota: 10, Burst: 10, Window: time.Minute},
			want: `10;w=60;comment="auth-ip"`,
		},
		{
			name: "burst 大于配额时显式写出",
			dim:  Dimension{ID: "global-ip", Quota: 100, Burst: 200, Window: time.Second},
			want: `100;w=1;burst=200;comment="global-ip"`,
		},
		{
			// w 只接受整数秒，写 0 会被客户端当作无限窗口。
			name: "亚秒窗口向上取整为 1 秒",
			dim:  Dimension{ID: "x", Quota: 5, Window: 200 * time.Millisecond},
			want: `5;w=1;comment="x"`,
		},
		{
			name: "无策略 id 时不写 comment",
			dim:  Dimension{Quota: 3, Window: time.Minute},
			want: "3;w=60",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.dim.Policy(); got != tc.want {
				t.Errorf("Policy() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWriteSetsEveryHeader(t *testing.T) {
	h := http.Header{}
	Write(h, Snapshot{
		Dimension: Dimension{ID: "global-ip", Quota: 100, Burst: 200, Window: time.Second},
		Remaining: 199,
		Reset:     750 * time.Millisecond,
	})

	// 成功响应也必须带齐四个字段，否则客户端只能等到 429 才知道配额。
	for _, name := range []string{HeaderLimit, HeaderRemaining, HeaderReset, HeaderPolicy} {
		if h.Get(name) == "" {
			t.Errorf("header %s not set", name)
		}
	}
	if got := h.Get(HeaderLimit); got != "200" {
		t.Errorf("RateLimit-Limit = %s, want bucket capacity 200", got)
	}
	if got := h.Get(HeaderReset); got != "1" {
		t.Errorf("RateLimit-Reset = %s, want 1 (relative seconds, rounded up)", got)
	}
	if h.Get(HeaderRetryAfter) != "" {
		t.Error("Retry-After must not be written by Write")
	}
}

func TestWriteClampsRemaining(t *testing.T) {
	cases := []struct {
		remaining int64
		want      string
	}{
		{remaining: -5, want: "0"},
		{remaining: 0, want: "0"},
		{remaining: 25, want: "25"},
		{remaining: 9999, want: "100"},
	}
	for _, tc := range cases {
		h := http.Header{}
		Write(h, Snapshot{
			Dimension: Dimension{Quota: 100, Window: time.Second},
			Remaining: tc.remaining,
		})
		if got := h.Get(HeaderRemaining); got != tc.want {
			t.Errorf("Remaining=%d → %s, want %s", tc.remaining, got, tc.want)
		}
	}
}

func TestWriteRetryAfterIsIntegerSeconds(t *testing.T) {
	cases := []struct {
		after time.Duration
		want  string
	}{
		{after: 1500 * time.Millisecond, want: "2"},
		{after: time.Second, want: "1"},
		// 写 0 会让客户端立即重试并放大压力。
		{after: 10 * time.Millisecond, want: "1"},
		{after: 0, want: "0"},
	}
	for _, tc := range cases {
		h := http.Header{}
		WriteRetryAfter(h, tc.after)
		if got := h.Get(HeaderRetryAfter); got != tc.want {
			t.Errorf("Retry-After(%s) = %s, want %s", tc.after, got, tc.want)
		}
	}
}

func TestConsume(t *testing.T) {
	dim := Dimension{ID: "auth-ip", Quota: 10, Burst: 5, Window: time.Minute}

	// 仍有令牌：无需等待。
	got := Consume(dim, 3.7, 10.0/60.0)
	if got.Remaining != 3 {
		t.Errorf("Remaining = %d, want 3 (floor)", got.Remaining)
	}
	if got.Reset != 0 {
		t.Errorf("Reset = %s, want 0 while tokens remain", got.Reset)
	}

	// 令牌耗尽：Reset 是恢复一个令牌所需时间（10 req/min → 6s），
	// 而不是伪造一个固定窗口边界。
	got = Consume(dim, 0, 10.0/60.0)
	if got.Reset != 6*time.Second {
		t.Errorf("Reset = %s, want 6s", got.Reset)
	}

	// 速率为 0（实际不恢复）时必须有界，否则客户端会死循环重试。
	got = Consume(dim, 0, 0)
	if got.Reset != dim.Window {
		t.Errorf("Reset = %s, want window %s as bounded fallback", got.Reset, dim.Window)
	}
}
