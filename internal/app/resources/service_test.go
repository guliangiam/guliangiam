package resources

import (
	"context"
	"io"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainresources "github.com/guliangiam/guliangiam/internal/domain/resources"
)

type fakeClock struct{ fixedTime time.Time }

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct{ fixedID string }

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

type mockStorage struct{}

func (m *mockStorage) Put(_ context.Context, _ string, _ []byte, _ string) error { return nil }
func (m *mockStorage) Get(_ context.Context, _ string) ([]byte, error)           { return nil, nil }
func (m *mockStorage) Delete(_ context.Context, _ string) error                  { return nil }
func (m *mockStorage) List(_ context.Context, _ string) ([]domainresources.ObjectInfo, error) {
	return nil, nil
}
func (m *mockStorage) GetURL(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", nil
}
func (m *mockStorage) PutStream(_ context.Context, _ string, _ io.Reader, _ string) error { return nil }

func TestService_WithStorage(t *testing.T) {
	var uow appuow.UnitOfWork
	svc := NewService(fakeClock{fixedTime: time.Now()}, fakeUUIDGenerator{fixedID: "test"}, uow, &mockStorage{}, nil)
	if svc.Storage == nil {
		t.Error("Storage should not be nil")
	}
}
