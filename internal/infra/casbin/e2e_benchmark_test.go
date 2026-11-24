package casbin

import (
	"strings"
	"testing"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/persist"
)

// ── Minimal 内存 Adapter（benchmark 专用，零 DB / 零 文件 I/O）──

// memoryAdapter 是 persist.Adapter 的最简实现：
// LoadPolicy 从预先构造的 policy 字符串加载，其他方法空实现。
// 不需要线程安全（benchmark 用 RunParallel 时每个 goroutine 独立 enforcer）。
type memoryAdapter struct {
	policy string
}

func (a *memoryAdapter) LoadPolicy(m model.Model) error {
	for _, line := range strings.Split(a.policy, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := persist.LoadPolicyLine(line, m); err != nil {
			return err
		}
	}
	return nil
}

func (a *memoryAdapter) SavePolicy(model.Model) error                              { return nil }
func (a *memoryAdapter) AddPolicy(string, string, []string) error                  { return nil }
func (a *memoryAdapter) RemovePolicy(string, string, []string) error               { return nil }
func (a *memoryAdapter) RemoveFilteredPolicy(string, string, int, ...string) error { return nil }

// ── Model + Policy 构造 ──

// 项目标准 RBAC with domain model（与 production DB 文本一致）
const benchModelRBACWithDomain = `
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act)
`

// genPolicy 生成 benchmark policy 规则
// numUsers × numRoles × numOrgs 规模；resources/methods 差异化授权
func genPolicy(numUsers, numRoles, numOrgs int) string {
	var sb strings.Builder

	resources := []string{
		"/api/v1/users", "/api/v1/users/*",
		"/api/v1/applications", "/api/v1/applications/*",
		"/api/v1/policies", "/api/v1/policies/*",
		"/api/v1/audit", "/api/v1/tokens",
		"/oauth2/*", "/.well-known/*",
	}
	methods := []string{"GET", "POST", "PUT", "DELETE", "(GET|POST)"}

	// g 规则：user → role（per org）
	for org := 0; org < numOrgs; org++ {
		for u := 0; u < numUsers; u++ {
			sb.WriteString("g, user")
			sb.WriteString(itoa(u))
			sb.WriteString(", role")
			sb.WriteString(itoa(u % numRoles))
			sb.WriteString(", org")
			sb.WriteString(itoa(org))
			sb.WriteString("\n")
		}
	}

	// p 规则：role → obj, act（差异化，不是所有 role 全开放）
	for org := 0; org < numOrgs; org++ {
		for r := 0; r < numRoles; r++ {
			for i, res := range resources {
				for j, m := range methods {
					if (r+i+j)%2 != 0 {
						continue
					}
					sb.WriteString("p, role")
					sb.WriteString(itoa(r))
					sb.WriteString(", org")
					sb.WriteString(itoa(org))
					sb.WriteString(", ")
					sb.WriteString(res)
					sb.WriteString(", ")
					sb.WriteString(m)
					sb.WriteString("\n")
				}
			}
		}
	}

	return sb.String()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

// newBenchEnforcer 构造真实 Enforcer（内存 adapter）
// 与 production manager.go 路径一致：NewModelFromString → NewEnforcer(m, adapter)
func newBenchEnforcer(policy string) (*casbin.Enforcer, error) {
	m, err := model.NewModelFromString(benchModelRBACWithDomain)
	if err != nil {
		return nil, err
	}
	adapter := &memoryAdapter{policy: policy}
	return casbin.NewEnforcer(m, adapter)
}

// ── 5 条 Benchmark ──

// B1: 小规模（4 用户 / 2 角色 / 2 域 —— 开发环境常态）
func BenchmarkEnforce_E2E_Small(b *testing.B) {
	e, err := newBenchEnforcer(genPolicy(4, 2, 2))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ok, _ := e.Enforce("user0", "org0", "/api/v1/users", "GET")
		if !ok {
			b.Fatal("expected allow for user0 GET /api/v1/users on org0")
		}
	}
}

// B2: 中规模（100 用户 / 5 角色 / 3 域 —— 生产小 org）
func BenchmarkEnforce_E2E_Medium(b *testing.B) {
	e, err := newBenchEnforcer(genPolicy(100, 5, 3))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		user := "user" + itoa(i%100)
		org := "org" + itoa(i%3)
		if _, err := e.Enforce(user, org, "/api/v1/applications", "GET"); err != nil {
			b.Fatal(err)
		}
	}
}

// B3: 大规模（1000 用户 / 10 角色 / 5 域 —— 生产大 org）
func BenchmarkEnforce_E2E_Large(b *testing.B) {
	e, err := newBenchEnforcer(genPolicy(1000, 10, 5))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		user := "user" + itoa(i%1000)
		org := "org" + itoa(i%5)
		if _, err := e.Enforce(user, org, "/api/v1/policies/*", "(GET|POST)"); err != nil {
			b.Fatal(err)
		}
	}
}

// B4: 通配符资源（keyMatch2 "/api/v1/users/42" → "/api/v1/users/*"）
// keyMatch2 是 matcher 里最耗时的函数
func BenchmarkEnforce_E2E_Wildcard(b *testing.B) {
	e, err := newBenchEnforcer(genPolicy(50, 3, 2))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := e.Enforce("user0", "org0", "/api/v1/users/42", "GET"); err != nil {
			b.Fatal(err)
		}
	}
}

// B5: goroutine 并发（模拟高并发 PEP 中间件）
func BenchmarkEnforce_E2E_Concurrent(b *testing.B) {
	e, err := newBenchEnforcer(genPolicy(100, 5, 3))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			user := "user" + itoa(i%100)
			org := "org" + itoa(i%3)
			if _, err := e.Enforce(user, org, "/api/v1/tokens", "GET"); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}
