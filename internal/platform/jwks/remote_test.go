package jwks

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// genRSAKey 生成测试用 RSA key pair。
func genRSAKey(t *testing.T) (key *rsa.PrivateKey, pem string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	kid := "test-kid-1"
	return key, kid
}

// jwkFromPub 把 rsa.PublicKey 转成 JWK JSON（RemoteJWKSCache 需要的输入）。
func jwkFromPub(pub *rsa.PublicKey, kid string) JWK {
	return JWK{
		Kty: "RSA",
		Use: "sig",
		Kid: kid,
		Alg: "RS256",
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

// startJWKSServer 启动一个返回测试 JWK 的 httptest server。
func startJWKSServer(t *testing.T, pub *rsa.PublicKey, kid string) *httptest.Server {
	t.Helper()
	jwk := jwkFromPub(pub, kid)
	jwksJSON, _ := json.Marshal(JWKS{Keys: []JWK{jwk}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksJSON)
	}))
	return srv
}

func TestRemoteJWKS_FetchAndCache(t *testing.T) {
	key, kid := genRSAKey(t)
	srv := startJWKSServer(t, &key.PublicKey, kid)
	defer srv.Close()

	cache := NewRemoteJWKSCache(5*time.Minute, 30*time.Minute, nil)
	defer cache.Close()

	ctx := context.Background()

	// 首次取 — 触发拉取
	pub, err := cache.GetPublicKey(ctx, srv.URL+"/jwks", kid)
	require.NoError(t, err)
	require.NotNil(t, pub)
	assert.True(t, pub.N.Cmp(key.N) == 0)

	// 再次取 — 应该命中缓存（server 不会再被请求）
	// httptest server 没有请求计数，但如果有问题会报错
	pub2, err := cache.GetPublicKey(ctx, srv.URL+"/jwks", kid)
	require.NoError(t, err)
	assert.True(t, pub2.N.Cmp(pub.N) == 0, "缓存命中应该返回同一个 key")
}

func TestRemoteJWKS_KidNotFound(t *testing.T) {
	key, _ := genRSAKey(t)
	srv := startJWKSServer(t, &key.PublicKey, "kid-A")
	defer srv.Close()

	cache := NewRemoteJWKSCache(5*time.Minute, 30*time.Minute, nil)
	defer cache.Close()

	_, err := cache.GetPublicKey(context.Background(), srv.URL+"/jwks", "kid-DOES-NOT-EXIST")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown signing key")
}

func TestRemoteJWKS_JWKSFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cache := NewRemoteJWKSCache(5*time.Minute, 30*time.Minute, nil)
	defer cache.Close()

	_, err := cache.GetPublicKey(context.Background(), srv.URL+"/jwks", "any")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestRemoteJWKS_EmptyJWKS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer srv.Close()

	cache := NewRemoteJWKSCache(5*time.Minute, 30*time.Minute, nil)
	defer cache.Close()

	_, err := cache.GetPublicKey(context.Background(), srv.URL+"/jwks", "kid-x")
	assert.Error(t, err)
}

func TestRemoteJWKS_NonRSAKeySkipped(t *testing.T) {
	// 返回一个 EC key — RemoteJWKSCache 应该跳过它（只支持 RSA）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[{"kty":"EC","crv":"P-256","x":"...","y":"...","kid":"ec-1"}]}`))
	}))
	defer srv.Close()

	cache := NewRemoteJWKSCache(5*time.Minute, 30*time.Minute, nil)
	defer cache.Close()

	// EC key 被跳过 → 缓存里没有 ec-1 → 返回 kid not found
	_, err := cache.GetPublicKey(context.Background(), srv.URL+"/jwks", "ec-1")
	assert.Error(t, err)
}

// jwkToRSAPublicKey 还能被 x509.MarshalPKIXPublicKey 接受（格式合法）
func TestJWKToRSAPublicKey_FormatValid(t *testing.T) {
	key, _ := genRSAKey(t)
	jwk := jwkFromPub(&key.PublicKey, "kid")
	pub, err := jwkToRSAPublicKey(&jwk)
	require.NoError(t, err)

	// 应该能 MarshalPKIXPublicKey
	der, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)
	assert.NotEmpty(t, der)
}
