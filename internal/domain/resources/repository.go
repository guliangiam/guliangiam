package resources

import (
	"context"
	"errors"
	"io"
	"time"
)

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	ApplicationID  string
	Keyword        string
	Type           string
	Tag            string
}

type Repository interface {
	// GetByID returns the resource only when it belongs to organizationID; a
	// foreign or missing row is reported as not found.
	GetByID(ctx context.Context, id, organizationID string) (*Resource, error)
	List(ctx context.Context, query ListQuery) ([]*Resource, int64, error)
	Create(ctx context.Context, resource *Resource) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, resource *Resource) error
	// Delete removes the resource only when it belongs to organizationID; zero
	// affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}

type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ContentType  string
}

type StorageProvider interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
	GetURL(ctx context.Context, key string, expires time.Duration) (string, error)
	PutStream(ctx context.Context, key string, reader io.Reader, contentType string) error
}

// ErrStorageProviderUnconfigured reports that the organization has no enabled
// storage provider row. Callers fall back to the deployment-level backend, the
// same way the messaging resolver falls back to environment senders.
var ErrStorageProviderUnconfigured = errors.New("storage provider unconfigured")

// StorageProviderResolver resolves the organization-scoped storage backend.
// A configured-but-disabled or malformed row must return a 503-class error,
// never the unconfigured sentinel: the two cases differ in whether an
// operator fallback is acceptable.
type StorageProviderResolver interface {
	Resolve(ctx context.Context, organizationID string) (StorageProvider, error)
}
