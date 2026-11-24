package sessions

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────
// P11 Benchmark — Session Lookup (Repository.GetByID) 端到端
//
// 与已有 BenchmarkSessionCreation / ExpiryCheck（纯内存 struct op）不同，
// 这里测的是 transport 层中间件的真实热点：
//
//   HTTP Request → Cookie 提取 sessionID → Repository.GetByID(id)
//              → expiry check → SessionContext 注入请求上下文
//
// Repository.GetByID 的真实 production 实现走 Postgres 查询（SELECT * FROM
// sessions WHERE id = $1），延迟由 DB 决定。这里测 in-memory map 实现，
// 得到 Go 层最小开销 + 规模敏感曲线（100 / 1k / 10k / 100k sessions）。
//
// 参考：spec §4.1 Session 恢复 P99 < 20ms（含 DB 查询）
// 这里测的是 DB 查询返回后 Go 层 GetByID 的开销 → 应该 << 1ms
// ─────────────────────────────────────────────────────────────────────

// ── In-Memory Repository（benchmark 专用，实现 Repository 接口）──

type benchRepo struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func newBenchRepo(count int) *benchRepo {
	r := &benchRepo{sessions: make(map[string]*Session, count)}
	now := time.Now()
	for i := 0; i < count; i++ {
		id := "sess-" + strconv.Itoa(i)
		r.sessions[id] = &Session{
			ID:               id,
			UserID:           "user-" + strconv.Itoa(i%100),
			Username:         "user" + strconv.Itoa(i%100),
			OrganizationID:   "org-" + strconv.Itoa(i%3),
			OrganizationSlug: "org" + strconv.Itoa(i%3),
			ApplicationID:    "app-" + strconv.Itoa(i%10),
			ApplicationSlug:  "app" + strconv.Itoa(i%10),
			SessionState:     "active",
			Sid:              "oidc-sid-" + strconv.Itoa(i),
			DeviceInfo:       map[string]any{"ua": "chrome/120", "os": "macos"},
			IPAddress:        "192.168.1." + strconv.Itoa(i%254),
			UserAgent:        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
			ExpiresAt:        now.Add(24 * time.Hour),
			CreatedAt:        now,
			LastSeenAt:       now,
			UpdatedAt:        now,
			Revision:         1,
		}
	}
	return r
}

// GetByID 是 transport 中间件的热点路径。
// 这里用 RLock/RUnlock（真实 DB 查询是 I/O 密集型，in-memory 是 CPU 密集型）。
func (r *benchRepo) GetByID(ctx context.Context, id string) (*Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	if !ok {
		return nil, errors.New("session not found")
	}
	return s, nil
}

// Repository 接口的其他方法（benchmark 不需要 GetByID 之外的）
func (r *benchRepo) List(context.Context, ListQuery) ([]*Session, int64, error) { return nil, 0, nil }
func (r *benchRepo) Create(context.Context, *Session) error                     { return nil }
func (r *benchRepo) Update(context.Context, *Session) error                     { return nil }
func (r *benchRepo) Touch(context.Context, string, time.Time) error             { return nil }
func (r *benchRepo) Revoke(context.Context, string, time.Time) error            { return nil }
func (r *benchRepo) RevokeByUserExcept(context.Context, string, string, time.Time) (int64, error) {
	return 0, nil
}
func (r *benchRepo) RevokeByUserAndApp(context.Context, string, string, string, time.Time) (int64, error) {
	return 0, nil
}
func (r *benchRepo) RevokeByUser(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}
func (r *benchRepo) RevokeByOrganization(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}
func (r *benchRepo) Delete(context.Context, string) error                     { return nil }
func (r *benchRepo) CleanupExpired(context.Context, time.Time) (int64, error) { return 0, nil }

// ── 4 条 Benchmark（规模敏感曲线）──

// B1: 100 sessions（demo / dev 环境）
func BenchmarkSessionLookup_100(b *testing.B) {
	benchmarkSessionLookup(b, 100)
}

// B2: 1000 sessions（生产小 org）
func BenchmarkSessionLookup_1K(b *testing.B) {
	benchmarkSessionLookup(b, 1000)
}

// B3: 10000 sessions（生产大 org）
func BenchmarkSessionLookup_10K(b *testing.B) {
	benchmarkSessionLookup(b, 10000)
}

// B4: 100000 sessions（超大规模 / 历史累积）
func BenchmarkSessionLookup_100K(b *testing.B) {
	benchmarkSessionLookup(b, 100000)
}

// B5: 并发 lookup（模拟高并发中间件）
func BenchmarkSessionLookup_Concurrent(b *testing.B) {
	repo := newBenchRepo(1000)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			id := "sess-" + strconv.Itoa(i%1000)
			_, err := repo.GetByID(ctx, id)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// B6: miss（session 已过期 / 不存在 —— 返回 ErrSessionNotFound）
func BenchmarkSessionLookup_Miss(b *testing.B) {
	repo := newBenchRepo(1000)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = repo.GetByID(ctx, "sess-nonexistent-"+strconv.Itoa(i))
	}
}

func benchmarkSessionLookup(b *testing.B, size int) {
	repo := newBenchRepo(size)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id := "sess-" + strconv.Itoa(i%size)
		s, err := repo.GetByID(ctx, id)
		if err != nil {
			b.Fatal(err)
		}
		// 关键：模拟 transport middleware 还要做的额外工作
		// （expiry check + state check —— 与 SessionCreation 同量级）
		_ = s.ExpiresAt.After(time.Now()) && s.RevokedAt == nil
		_ = s.SessionState == "active"
	}
}
