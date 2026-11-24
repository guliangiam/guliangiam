package hibp

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hashParts(t *testing.T, password string) (prefix, suffix string) {
	t.Helper()
	sum := sha1.Sum([]byte(password))
	h := strings.ToUpper(hex.EncodeToString(sum[:]))
	return h[:5], h[5:]
}

func errorCode(t *testing.T, err error) apperrors.Code {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, stderrors.As(err, &appErr), "expected apperrors.Error, got %T: %v", err, err)
	return appErr.Code
}

// k-匿名关闭条件的核心断言：出域请求只携带 SHA-1 前 5 位，明文与完整哈希不出进程。
func TestIsBreached_OnlyPrefixLeavesProcess(t *testing.T) {
	password := "correct horse battery staple"
	prefix, suffix := hashParts(t, password)
	fullHash := prefix + suffix

	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		assert.Equal(t, http.MethodGet, r.Method)
		fmt.Fprintf(w, "%s:7\r\nOTHERSUFFIX:1\r\n", suffix)
	}))
	defer srv.Close()

	c := NewRangeChecker(srv.URL, 2*time.Second, nil)
	breached, err := c.IsBreached(context.Background(), password)
	require.NoError(t, err)
	assert.True(t, breached)

	assert.Equal(t, "/range/"+prefix, gotPath)
	assert.Empty(t, gotQuery)
	assert.NotContains(t, gotPath, fullHash)
	assert.NotContains(t, gotPath, password)
	assert.NotContains(t, gotPath, suffix)
}

func TestIsBreached_Clean(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s:3\r\n", strings.Repeat("0", 35))
	}))
	defer srv.Close()

	c := NewRangeChecker(srv.URL, 2*time.Second, nil)
	breached, err := c.IsBreached(context.Background(), "a-strong-unique-password")
	require.NoError(t, err)
	assert.False(t, breached)
}

// Add-Padding 填充行 count=0：命中填充行不得误判为泄露。
func TestIsBreached_PaddingEntryDoesNotMatch(t *testing.T) {
	_, suffix := hashParts(t, "padding-guard-password")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s:0\r\n", suffix)
	}))
	defer srv.Close()

	c := NewRangeChecker(srv.URL, 2*time.Second, nil)
	breached, err := c.IsBreached(context.Background(), "padding-guard-password")
	require.NoError(t, err)
	assert.False(t, breached)
}

func TestIsBreached_RequestHeaders(t *testing.T) {
	var gotUA, gotPadding string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotPadding = r.Header.Get("Add-Padding")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewRangeChecker(srv.URL, 2*time.Second, nil)
	_, err := c.IsBreached(context.Background(), "header-check")
	require.NoError(t, err)
	assert.Equal(t, "GuliangIAM", gotUA)
	assert.Equal(t, "true", gotPadding)
}

func TestIsBreached_Non200IsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewRangeChecker(srv.URL, 2*time.Second, nil)
	_, err := c.IsBreached(context.Background(), "origin-down")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnavailable, errorCode(t, err))
}

func TestIsBreached_CacheHitAvoidsSecondHTTP(t *testing.T) {
	_, suffix := hashParts(t, "cached-password")
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		fmt.Fprintf(w, "%s:2\r\n", suffix)
	}))
	defer srv.Close()

	c := NewRangeChecker(srv.URL, 2*time.Second, NewLocalRangeCache())
	ctx := context.Background()
	for range 2 {
		breached, err := c.IsBreached(ctx, "cached-password")
		require.NoError(t, err)
		assert.True(t, breached)
	}
	assert.Equal(t, 1, hits, "第二次查询必须命中缓存，不再出域")
}

type brokenCache struct{}

func (brokenCache) Get(context.Context, string) (string, bool, error) {
	return "", false, apperrors.NewUnavailable("cache down")
}
func (brokenCache) Set(context.Context, string, string) error {
	return apperrors.NewUnavailable("cache down")
}

// 缓存故障不得独立失败：回源成功即正常判定。
func TestIsBreached_CacheErrorFallsThroughToOrigin(t *testing.T) {
	_, suffix := hashParts(t, "cache-down-password")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s:9\r\n", suffix)
	}))
	defer srv.Close()

	c := NewRangeChecker(srv.URL, 2*time.Second, brokenCache{})
	breached, err := c.IsBreached(context.Background(), "cache-down-password")
	require.NoError(t, err)
	assert.True(t, breached)
}

func TestNewRangeChecker_Defaults(t *testing.T) {
	c := NewRangeChecker("", 2*time.Second, nil)
	assert.Equal(t, defaultBaseURL, c.BaseURL)

	c = NewRangeChecker("https://mirror.example.com/", 2*time.Second, nil)
	assert.Equal(t, "https://mirror.example.com", c.BaseURL)
}
