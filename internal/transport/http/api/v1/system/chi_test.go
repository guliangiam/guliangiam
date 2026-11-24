package system

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	return r
}

func doRequest(t *testing.T, router chi.Router, method, path string, body io.Reader) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Result()
}

func TestRealRoutes_Health(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, nil)
	resp := doRequest(t, router, http.MethodGet, "/api/v1/system/health", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

// 上一轮 /system-info 前后端契约漂移（字段被重命名）正是从「只断言非 0 状态」漏过。
// 以下用例锁定 JSON 键集合与类型，任何静默重命名都会失败。
func decodeObject(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body
}

func requireStringKeys(t *testing.T, body map[string]any, keys ...string) {
	t.Helper()
	for _, key := range keys {
		value, ok := body[key]
		require.Truef(t, ok, "missing key %q in %v", key, body)
		_, isString := value.(string)
		require.Truef(t, isString, "key %q must be a string, got %T", key, value)
	}
}

func TestSystemVersion_ResponseContract(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, nil)
	resp := doRequest(t, router, http.MethodGet, "/api/v1/system/version", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeObject(t, resp)

	require.Len(t, body, 6)
	requireStringKeys(t, body, "version", "buildTime", "commitHash", "goVersion", "os", "arch")
}

func TestSystemInfo_ResponseContract(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, nil)
	resp := doRequest(t, router, http.MethodGet, "/api/v1/system/info", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeObject(t, resp)

	require.Len(t, body, 8)
	for _, key := range []string{"goVersion", "os", "arch"} {
		value, ok := body[key]
		require.Truef(t, ok, "missing key %q", key)
		_, isString := value.(string)
		require.Truef(t, isString, "key %q must be a string, got %T", key, value)
	}
	for _, key := range []string{"numCPU", "numGoroutine", "heapAlloc", "heapSys", "stackInuse"} {
		value, ok := body[key]
		require.Truef(t, ok, "missing key %q", key)
		_, isNumber := value.(float64)
		require.Truef(t, isNumber, "key %q must be a number, got %T", key, value)
	}
}

func TestSystemHealth_ResponseContract(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, nil)
	resp := doRequest(t, router, http.MethodGet, "/api/v1/system/health", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeObject(t, resp)

	require.Len(t, body, 1)
	require.Equal(t, "ok", body["status"])
}

func TestSystemReady_NilPoolDegradesToLiveness(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, nil)
	resp := doRequest(t, router, http.MethodGet, "/api/v1/system/ready", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeObject(t, resp)

	require.Len(t, body, 1)
	require.Equal(t, "ok", body["status"])
}

func TestRealRoutes_Info(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, nil)
	resp := doRequest(t, router, http.MethodGet, "/api/v1/system/info", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_Version(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, nil)
	resp := doRequest(t, router, http.MethodGet, "/api/v1/system/version", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_Metrics(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, nil)
	resp := doRequest(t, router, http.MethodGet, "/api/v1/system/metrics", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}

func TestRealRoutes_PrometheusInfo(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, nil)
	resp := doRequest(t, router, http.MethodGet, "/api/v1/system/prometheus-info", nil)
	assert.NotEqual(t, 0, resp.StatusCode)
}
