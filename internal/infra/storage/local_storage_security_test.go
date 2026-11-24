package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// local_storage_security_test.go 锁定本地存储的路径边界：键来自调用方（对象
// 存储语义），必须拒绝绝对路径、向上回溯与符号链接逃逸，同时保留合法嵌套键。

func TestLocalStorage_RejectsUnsafeKeys(t *testing.T) {
	s := NewLocalStorage(t.TempDir(), "http://cdn.example.test")
	unsafeKeys := []string{
		"/etc/passwd",
		"../etc/passwd",
		"nested/../../etc/passwd",
		"..",
	}
	for _, key := range unsafeKeys {
		t.Run(key, func(t *testing.T) {
			_, err := s.Get(context.Background(), key)
			require.Error(t, err, "Get must reject key %q", key)

			err = s.Put(context.Background(), key, []byte("x"), "text/plain")
			require.Error(t, err, "Put must reject key %q", key)

			err = s.Delete(context.Background(), key)
			require.Error(t, err, "Delete must reject key %q", key)

			_, err = s.GetURL(context.Background(), key, time.Hour)
			require.Error(t, err, "GetURL must reject key %q", key)
		})
	}
}

func TestLocalStorage_RejectsSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600))
	if err := os.Symlink(outside, filepath.Join(base, "escape")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	s := NewLocalStorage(base, "http://cdn.example.test")

	_, err := s.Get(context.Background(), "escape/secret.txt")
	require.Error(t, err, "existing file behind an escaping symlink must not be readable")

	err = s.Put(context.Background(), "escape/new.txt", []byte("x"), "text/plain")
	require.Error(t, err, "write through an escaping symlink must be rejected")

	err = s.PutStream(context.Background(), "escape/stream.txt", strings.NewReader("x"), "text/plain")
	require.Error(t, err, "stream write through an escaping symlink must be rejected")

	_, err = s.List(context.Background(), "escape")
	require.Error(t, err, "listing an escaping symlink target must be rejected")
}

func TestLocalStorage_AllowsSymlinkInsideBase(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(base, "real"), 0o755))
	if err := os.Symlink(filepath.Join(base, "real"), filepath.Join(base, "alias")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	s := NewLocalStorage(base, "http://cdn.example.test")
	require.NoError(t, s.Put(context.Background(), "alias/inside.txt", []byte("ok"), "text/plain"))

	got, err := s.Get(context.Background(), "alias/inside.txt")
	require.NoError(t, err)
	require.Equal(t, []byte("ok"), got)
}

func TestLocalStorage_AllowsNestedKeys(t *testing.T) {
	s := NewLocalStorage(t.TempDir(), "http://cdn.example.test")
	key := "org-1/avatars/user-42.png"
	require.NoError(t, s.Put(context.Background(), key, []byte("png"), "image/png"))

	got, err := s.Get(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, []byte("png"), got)

	url, err := s.GetURL(context.Background(), key, time.Hour)
	require.NoError(t, err)
	require.Contains(t, url, key)

	require.NoError(t, s.Delete(context.Background(), key))
}
