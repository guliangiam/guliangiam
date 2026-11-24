package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// localStorage stores objects on the local filesystem.
type localStorage struct {
	baseDir string
	baseURL string
}

// NewLocalStorage creates a new local filesystem storage provider.
func NewLocalStorage(baseDir, baseURL string) Provider {
	return &localStorage{baseDir: baseDir, baseURL: baseURL}
}

// safePath validates the key and returns an absolute path guaranteed to stay
// inside baseDir. Keys are rejected when absolute or when they traverse upwards;
// the resolved path is also checked after expanding symlinks so a link inside
// the base directory cannot redirect reads/writes outside it.
func (s *localStorage) safePath(key string) (string, error) {
	if filepath.IsAbs(key) {
		return "", apperrors.NewInvalidArgument("storage key must not be absolute path")
	}
	cleaned := filepath.Clean(key)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", apperrors.NewInvalidArgument("storage key contains path traversal")
	}

	base, err := filepath.Abs(s.baseDir)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "resolve base dir")
	}
	path, err := filepath.Abs(filepath.Join(base, cleaned))
	if err != nil {
		return "", apperrors.WrapInternalf(err, "resolve path")
	}

	resolvedBase, err := resolveExistingPrefix(base)
	if err != nil {
		return "", err
	}
	resolvedPath, err := resolveExistingPrefix(path)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(resolvedBase, resolvedPath)
	if err != nil || rel == ".." || filepath.IsAbs(rel) ||
		strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", apperrors.NewInvalidArgument("storage key escapes base directory")
	}
	return resolvedPath, nil
}

// resolveExistingPrefix expands symlinks on the deepest existing ancestor of
// path and re-appends the components that do not exist yet. A component that
// has not been created cannot be a symlink, but its existing ancestors can.
func resolveExistingPrefix(path string) (string, error) {
	existing := path
	var missing []string
	for {
		_, err := os.Lstat(existing)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", apperrors.WrapInternalf(err, "inspect storage path")
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", apperrors.NewInvalidArgument("storage path has no existing ancestor")
		}
		missing = append(missing, filepath.Base(existing))
		existing = parent
	}

	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "resolve storage symlink")
	}
	for i := len(missing) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, missing[i])
	}
	return resolved, nil
}

func (s *localStorage) Put(ctx context.Context, key string, data []byte, contentType string) error {
	path, err := s.safePath(key)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return apperrors.WrapInternalf(err, "local storage mkdir")
	}
	// 上传对象需要被本地静态服务/其他进程读取，0o644 是本地对象存储的预期权限。
	if err := os.WriteFile(path, data, 0o644); err != nil { //nolint:gosec // G306: object files are served locally and must be group/world readable
		return apperrors.WrapInternalf(err, "local storage write")
	}
	return nil
}

func (s *localStorage) Get(ctx context.Context, key string) ([]byte, error) {
	path, err := s.safePath(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "local storage read")
	}
	return data, nil
}

func (s *localStorage) Delete(ctx context.Context, key string) error {
	path, err := s.safePath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return apperrors.WrapInternalf(err, "local storage delete")
	}
	return nil
}

func (s *localStorage) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	dir, err := s.safePath(prefix)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, apperrors.WrapInternalf(err, "local storage list")
	}

	var infos []ObjectInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		infos = append(infos, ObjectInfo{
			Key:          filepath.Join(prefix, entry.Name()),
			Size:         info.Size(),
			LastModified: info.ModTime(),
		})
	}
	return infos, nil
}

func (s *localStorage) GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	if _, err := s.safePath(key); err != nil {
		return "", err
	}
	return s.baseURL + "/" + key, nil
}

func (s *localStorage) PutStream(ctx context.Context, key string, reader io.Reader, contentType string) error {
	path, err := s.safePath(key)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return apperrors.WrapInternalf(err, "local storage mkdir")
	}
	f, err := os.Create(path)
	if err != nil {
		return apperrors.WrapInternalf(err, "local storage create")
	}
	defer f.Close()
	if _, err := io.Copy(f, reader); err != nil {
		return apperrors.WrapInternalf(err, "local storage copy")
	}
	return nil
}
