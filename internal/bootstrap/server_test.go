package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
)

type federationCacheCloserStub struct{ closeCalls int }

func (s *federationCacheCloserStub) Close() { s.closeCalls++ }

func TestConfigureFederationWiresVerifierAndUOWResolver(t *testing.T) {
	service := appauth.NewService(&fakeUnitOfWork{})
	cache := configureFederation(&service)
	if cache == nil {
		t.Fatal("configureFederation() cache = nil")
	}
	cache.Close()

	fields := reflect.ValueOf(&service).Elem()
	if fields.FieldByName("federationVerifier").IsNil() {
		t.Error("federationVerifier was not injected")
	}
	if fields.FieldByName("federationResolver").IsNil() {
		t.Error("federationResolver was not injected")
	}
}

func TestRuntimeCloseClosesFederationJWKSCache(t *testing.T) {
	cache := &federationCacheCloserStub{}
	runtime := &Runtime{Lifecycle: LifecycleConfig{FederationJWKSCache: cache}}
	runtime.Close()
	if cache.closeCalls != 1 {
		t.Fatalf("FederationJWKSCache.Close() calls = %d, want 1", cache.closeCalls)
	}
}

func TestResolveHTTPAddressUsesAddrEnvFirst(t *testing.T) {
	t.Setenv("HTTP_ADDR", "127.0.0.1:19090")
	t.Setenv("HTTP_PORT", "19091")

	if got := ResolveHTTPAddress(); got != "127.0.0.1:19090" {
		t.Fatalf("ResolveHTTPAddress() = %q, want %q", got, "127.0.0.1:19090")
	}
}

func TestResolveHTTPAddressUsesPortEnv(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("HTTP_PORT", "19092")

	if got := ResolveHTTPAddress(); got != ":19092" {
		t.Fatalf("ResolveHTTPAddress() = %q, want %q", got, ":19092")
	}
}

func TestRegisterSystemRoutesHealthz(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json; charset=utf-8")
	}

	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf("status = %q, want %q", body["status"], "ok")
	}
}

func TestRegisterSystemRoutesOpenAPI(t *testing.T) {
	router := chi.NewRouter()
	registerSystemRoutes(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/openapi/openapi.yaml", http.NoBody)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if got := recorder.Header().Get("Content-Type"); got != "application/yaml; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/yaml; charset=utf-8")
	}

	if len(recorder.Body.Bytes()) == 0 {
		t.Fatalf("openapi body is empty")
	}

	if got := recorder.Header().Get("ETag"); got == "" {
		t.Fatalf("ETag header is empty")
	}
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=300" {
		t.Fatalf("Cache-Control = %q, want %q", got, "public, max-age=300")
	}
}

func TestRegisterSystemRoutesOpenAPIJSON(t *testing.T) {
	router := chi.NewRouter()
	registerSystemRoutes(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/openapi/openapi.json", http.NoBody)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json; charset=utf-8")
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if body["openapi"] != "3.1.0" {
		t.Fatalf("openapi = %#v, want %#v", body["openapi"], "3.1.0")
	}

	paths, ok := body["paths"].(map[string]any)
	if !ok {
		t.Fatalf("paths missing from openapi json")
	}
	if _, ok := paths["/api/v1/auth/logout"]; !ok {
		t.Fatalf("/api/v1/auth/logout missing from openapi json")
	}

	components, ok := body["components"].(map[string]any)
	if !ok {
		t.Fatalf("components missing from openapi json")
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatalf("components.schemas missing from openapi json")
	}
	if _, ok := schemas["AuthAccountContext"]; !ok {
		t.Fatalf("AuthAccountContext missing from openapi json schemas")
	}

	if got := recorder.Header().Get("ETag"); got == "" {
		t.Fatalf("ETag header is empty")
	}
}

func TestRegisterSystemRoutesOpenAPIETagNotModified(t *testing.T) {
	router := chi.NewRouter()
	registerSystemRoutes(router)

	firstRecorder := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodGet, "/openapi/openapi.yaml", http.NoBody)
	router.ServeHTTP(firstRecorder, firstRequest)

	etag := firstRecorder.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("expected etag from initial request")
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/openapi/openapi.yaml", http.NoBody)
	request.Header.Set("If-None-Match", etag)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotModified {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusNotModified)
	}

	if recorder.Body.Len() != 0 {
		t.Fatalf("expected empty body for not modified response")
	}
}
