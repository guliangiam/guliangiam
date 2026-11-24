package bootstrap

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/platform/config"
	platformmtls "github.com/guliangiam/guliangiam/internal/platform/mtls"
	openapi "github.com/guliangiam/guliangiam/openapi"
)

func NewHTTPServer(addr string) (*http.Server, *Runtime, error) {
	if err := validateMTLSProxyConfig(); err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	runtime, err := NewRuntimeFromEnv(ctx)
	if err != nil {
		return nil, nil, err
	}

	router := NewChiRouter(runtime)
	registerSystemRoutes(router)

	server := &http.Server{
		Addr:    addr,
		Handler: router,
		// Timeout 三件套 — 防 Slowloris + 超时保护（架构 §8.6）
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
		// 显式固定为 1 MiB，不依赖版本的默认值（SEC-A5）。
		MaxHeaderBytes: 1 << 20,
	}

	certFile, keyFile := config.TLSCertFile(), config.TLSKeyFile()
	if (certFile == "") != (keyFile == "") {
		return nil, nil, errors.New("TLS_CERT_FILE and TLS_KEY_FILE must be set together")
	}
	// The CA bundle also serves the trusted-proxy mode, where the app layer
	// verifies the forwarded chain even though this process terminates no TLS.
	if config.TLSClientCAFile() != "" && certFile == "" && config.MTLSProxyClientCertHeader() == "" {
		return nil, nil, errors.New("TLS_CLIENT_CA_FILE requires TLS_CERT_FILE/TLS_KEY_FILE (in-process TLS termination) or MTLS_PROXY_CLIENT_CERT_HEADER")
	}
	if config.IsProduction() || certFile != "" {
		tlsConfig, err := productionTLSConfig()
		if err != nil {
			return nil, nil, err
		}
		server.TLSConfig = tlsConfig
		slog.Info("TLS configuration applied", "in_process", certFile != "", "client_ca", config.TLSClientCAFile() != "")
	}

	return server, runtime, nil
}

// validateMTLSProxyConfig rejects a forwarded-certificate mode that cannot be
// trusted. The header is only read for peers inside TRUSTED_PROXY_CIDRS, so an
// opt-in without that list would silently disable mTLS for every client.
func validateMTLSProxyConfig() error {
	header := config.MTLSProxyClientCertHeader()
	if header == "" {
		return nil
	}
	if config.TrustedProxyCIDRs() == "" {
		return errors.New("MTLS_PROXY_CLIENT_CERT_HEADER requires TRUSTED_PROXY_CIDRS")
	}
	if !config.ValidHeaderName(header) {
		return fmt.Errorf("MTLS_PROXY_CLIENT_CERT_HEADER %q is not a valid header name", header)
	}
	return nil
}

// productionTLSConfig 只配置标准 X.509 TLS 1.3 套件。国密 TLCP 终止是部署层
// 能力（gm_tls）：internal/platform/crypto 校验双证书材料后披露，本函数不接线。
func productionTLSConfig() (*tls.Config, error) {
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
	}
	// Always ask for a client certificate when TLS terminates in-process.
	// RequestClientCert (rather than VerifyClientCertIfGiven) keeps the handshake
	// usable for self_signed_tls_client_auth clients whose certificate has no
	// trusted chain — Go aborts the handshake on an unverifiable certificate
	// under VerifyClientCertIfGiven. This means the handshake itself never
	// rejects an untrusted client certificate: the complete trust decision
	// moves to the app layer (internal/app/auth/client_auth_mtls.go), which
	// (1) verifies the tls_client_auth chain against TLS_CLIENT_CA_FILE,
	// (2) compares the leaf's x5t#S256 byte-for-byte with a row registered for
	// the same method, and (3) fails closed when no CA is configured. Only when
	// all of these pass is the client authenticated, so a certificate the TLS
	// stack accepted but this chain cannot trust still ends in invalid_client.
	// Browser/API sessions are unaffected: the certificate is optional.
	cfg.ClientAuth = tls.RequestClientCert
	if caFile := config.TLSClientCAFile(); caFile != "" {
		pool, err := platformmtls.LoadCertPoolFromFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read TLS_CLIENT_CA_FILE: %w", err)
		}
		cfg.ClientCAs = pool
	}
	return cfg, nil
}

func writeOpenAPIDocument(w http.ResponseWriter, r *http.Request, contentType, etag string, body []byte) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("ETag", etag)
	if strings.TrimSpace(r.Header.Get("If-None-Match")) == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func registerSystemRoutes(router chi.Router) {
	// /metrics 由 RegisterOpsRoutes 在 ops 组单独挂（走 MetricsTokenMiddleware），
	// 此处不再重复注册 — P5-1 收敛。

	router.Get("/openapi/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		writeOpenAPIDocument(w, r, "application/yaml; charset=utf-8", openapi.ServerYAMLETag(), openapi.ServerYAML())
	})

	router.Get("/openapi/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		body, err := openapi.ServerJSON()
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"openapi_json_generation_failed"}`))
			return
		}

		tag, err := openapi.ServerJSONETag()
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"openapi_json_generation_failed"}`))
			return
		}

		writeOpenAPIDocument(w, r, "application/json; charset=utf-8", tag, body)
	})
}

func ResolveHTTPAddress() string {
	if value := strings.TrimSpace(config.HTTPAddr()); value != "" {
		return value
	}

	port := strings.TrimSpace(config.HTTPPort())
	if port != "" {
		if strings.HasPrefix(port, ":") {
			return port
		}
		return ":" + port
	}

	return ":8000"
}
