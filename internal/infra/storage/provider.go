package storage

import (
	"context"
	"io"
	"time"

	domainresources "github.com/guliangiam/guliangiam/internal/domain/resources"
)

type ObjectInfo = domainresources.ObjectInfo

// Provider is the interface for storage backends.
type Provider interface {
	// Put stores an object with the given key.
	Put(ctx context.Context, key string, data []byte, contentType string) error
	// Get retrieves an object by key.
	Get(ctx context.Context, key string) ([]byte, error)
	// Delete removes an object by key.
	Delete(ctx context.Context, key string) error
	// List lists objects with the given prefix.
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
	// GetURL returns a presigned URL for the object (if supported).
	GetURL(ctx context.Context, key string, expires time.Duration) (string, error)
	// PutStream stores an object from a reader (optional, for large files).
	PutStream(ctx context.Context, key string, reader io.Reader, contentType string) error
}
