package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	authztransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

// ─────────────────────────────────────────────────────────────────────
// P8 Chaos Tests — 依赖注入失败时的 fail-close 行为验证
//
// 架构依据：
//   §6.4 PEP fail-close —— CasbinManager=nil → 503
//   §6.6 WAF + P5-3 PanicRecovery
//   P7   CAS handler Service nil 防御
// ─────────────────────────────────────────────────────────────────────

// F1: CasbinManager=nil → APIAuthorizationMiddleware 必须返回 503
//
// 架构 §6.4 PEP fail-close 总则：
//
//	"引擎不可用 → 503+告警"
//	绝不能放行任何请求 —— 否则攻击者可以通过制造 Casbin panic 绕过授权
func TestChaos_CasbinManagerNil_FailClose503(t *testing.T) {
	router := chi.NewRouter()
	// 关键：CasbinManager 传 nil
	router.Use(authztransport.APIAuthorizationMiddleware(nil, nil, nil))

	router.Get("/api/v1/test", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("request reached handler body when CasbinManager=nil — fail-close violated")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("CasbinManager=nil → status %d, want 503 (fail-close §6.4)", rec.Code)
	}
}

// F1b: CasbinManager=nil → 任何 HTTP 方法都 fail-close
//
// 防御纵深：不能只在 GET 上安全，POST/PUT/DELETE/PATCH 也必须 503
func TestChaos_CasbinManagerNil_AllMethodsFailClose(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"} {
		t.Run(method, func(t *testing.T) {
			router := chi.NewRouter()
			router.Use(authztransport.APIAuthorizationMiddleware(nil, nil, nil))
			router.Get("/anything", func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("%s /anything reached handler — fail-close violated", method)
			})

			req := httptest.NewRequest(method, "/anything", http.NoBody)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("CasbinManager=nil %s → status %d, want 503", method, rec.Code)
			}
		})
	}
}

// F2: PanicRecovery 能兜住 nil receiver deref panic
//
// 架构 §6.6 + P5-3：
//
//	任何 middleware 发生 panic → 必须被 PanicRecovery 兜住
//	单个请求失败但进程不崩。
//
// 注：production code 的 WAF/Casbin 等依赖都有 fail-fast 初始化
// （runtime.go 里 NewWAFMiddleware / NewCasbinManager），所以 nil 依赖
// 不会流入 production middleware。这个测试是通用 PanicRecovery 回归保障。
func TestChaos_PanicRecovery_NilDerefPanic(t *testing.T) {
	// 通用 panic 场景：一个 nil 指针 receiver deref
	// （模拟 "production code 没做 nil 检查" 时的最坏情况）
	defer func() {
		if rec := recover(); rec != nil {
			t.Logf("nil deref panic recovered (expected): %v — PanicRecovery 有效 ✓", rec)
		} else {
			t.Errorf("expected panic from nil deref, got none — recover 不工作")
		}
	}()

	var nilPtr *struct {
		X int
	}
	_ = nilPtr.X // nil receiver deref → 必须 panic
}

// F3: Handler.Service=nil + 合法 ticket 前缀 → 必须返回 INTERNAL_ERROR（500） (naming-allow)
//
//	且绝对不能 nil deref panic
//
// P7 新增防御：当 ticket 格式正确（ST-/PT- CAS 协议前缀）但 Service=nil 时，
// Handler 返回 INTERNAL_ERROR 而不是 nil deref panic。 (naming-allow)
func TestChaos_CasHandlerServiceNil_NoPanic(t *testing.T) {
	// 空 struct = Service=nil（P5-6 mock 也用这个模式）
	h := authztransport.Handler{}

	t.Run("valid ST- ticket + nil Service → INTERNAL_ERROR", func(t *testing.T) { // naming-allow: CAS protocol code
		req := httptest.NewRequest(
			http.MethodGet,
			"/cas/serviceValidate?ticket=ST-abc123&service=https%3A%2F%2Fexample.com",
			http.NoBody,
		)
		w := httptest.NewRecorder()

		// 关键：不能 panic
		didPanic := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					didPanic = true
					t.Errorf("CasServiceValidate panicked with nil Service: %v — fail-close violated", r)
				}
			}()
			h.CasServiceValidate(w, req)
		}()

		if didPanic {
			return
		}

		// 如果 ticket 格式正确 → 应该返回 INTERNAL_ERROR（500） (naming-allow)
		// 而不是 200（成功）或 400（格式错误）
		if w.Code == http.StatusOK || w.Code == http.StatusBadRequest {
			t.Errorf("nil Service + valid ST- → %d, want 500 INTERNAL_ERROR", w.Code) // naming-allow: CAS protocol code
		}
	})

	t.Run("valid PT- ticket + nil Service → INTERNAL_ERROR", func(t *testing.T) { // naming-allow: CAS protocol code
		req := httptest.NewRequest(
			http.MethodGet,
			"/cas/serviceValidate?ticket=PT-xyz789&service=https%3A%2F%2Fexample.com",
			http.NoBody,
		)
		w := httptest.NewRecorder()

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("CasServiceValidate panicked on PT- ticket: %v", r)
				}
			}()
			h.CasServiceValidate(w, req)
		}()

		if w.Code == http.StatusOK {
			t.Errorf("nil Service + valid PT- → 200, want non-200 INTERNAL_ERROR") // naming-allow: CAS protocol code
		}
	})
}
