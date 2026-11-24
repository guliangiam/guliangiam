package storage

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockProvider struct{}

func (m *mockProvider) Put(ctx context.Context, key string, data []byte, contentType string) error {
	return nil
}
func (m *mockProvider) Get(ctx context.Context, key string) ([]byte, error) { return nil, nil }
func (m *mockProvider) Delete(ctx context.Context, key string) error        { return nil }
func (m *mockProvider) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	return nil, nil
}
func (m *mockProvider) GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	return "", nil
}
func (m *mockProvider) PutStream(ctx context.Context, key string, reader io.Reader, contentType string) error {
	return nil
}

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.Empty(t, f.Types())
}

func TestFactory_RegisterAndGet(t *testing.T) {
	f := NewFactory()
	p := &mockProvider{}
	f.Register("local", p)

	got, err := f.GetProvider("local")
	assert.NoError(t, err)
	assert.Equal(t, p, got)
}

func TestFactory_GetProvider_NotFound(t *testing.T) {
	f := NewFactory()
	_, err := f.GetProvider("s3")
	assert.Error(t, err)
}

func TestFactory_Verify_Registered(t *testing.T) {
	f := NewFactory()
	f.Register("local", &mockProvider{})
	err := f.Verify("local")
	assert.NoError(t, err)
}

func TestFactory_Verify_NotRegistered(t *testing.T) {
	f := NewFactory()
	err := f.Verify("s3")
	assert.Error(t, err)
}

func TestFactory_Types(t *testing.T) {
	f := NewFactory()
	f.Register("local", &mockProvider{})
	f.Register("s3", &mockProvider{})
	types := f.Types()
	assert.Len(t, types, 2)
}

func TestFactory_Register_Replace(t *testing.T) {
	f := NewFactory()
	p1 := &mockProvider{}
	p2 := &mockProvider{}
	f.Register("local", p1)
	f.Register("local", p2)
	got, err := f.GetProvider("local")
	assert.NoError(t, err)
	assert.Equal(t, p2, got)
}
