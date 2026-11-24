package storage

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLocalStorage_PutAndGet(t *testing.T) {
	s := NewLocalStorage(t.TempDir(), "http://localhost:8080")
	err := s.Put(context.Background(), "test.txt", []byte("hello world"), "text/plain")
	assert.NoError(t, err)

	data, err := s.Get(context.Background(), "test.txt")
	assert.NoError(t, err)
	assert.Equal(t, []byte("hello world"), data)
}

func TestLocalStorage_Delete(t *testing.T) {
	s := NewLocalStorage(t.TempDir(), "http://localhost:8080")
	_ = s.Put(context.Background(), "test.txt", []byte("hello"), "text/plain")
	err := s.Delete(context.Background(), "test.txt")
	assert.NoError(t, err)
}

func TestLocalStorage_List(t *testing.T) {
	s := NewLocalStorage(t.TempDir(), "http://localhost:8080")
	_ = s.Put(context.Background(), "file1.txt", []byte("a"), "text/plain")
	_ = s.Put(context.Background(), "file2.txt", []byte("b"), "text/plain")

	items, err := s.List(context.Background(), "")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(items), 2)
}

func TestLocalStorage_GetURL(t *testing.T) {
	s := NewLocalStorage(t.TempDir(), "http://localhost:8080")
	url, err := s.GetURL(context.Background(), "test.txt", time.Hour)
	assert.NoError(t, err)
	assert.NotEmpty(t, url)
}

func TestLocalStorage_PutStream(t *testing.T) {
	s := NewLocalStorage(t.TempDir(), "http://localhost:8080")
	err := s.PutStream(context.Background(), "stream.txt", bytes.NewReader([]byte("stream data")), "text/plain")
	assert.NoError(t, err)
}

func TestCUCloudEndpoint(t *testing.T) {
	result := cuCloudEndpoint("cn-north")
	assert.Equal(t, "oos-cn-north.ctyunapi.cn", result)

	result = cuCloudEndpoint(" CN-SOUTH ")
	assert.Equal(t, "oos-cn-south.ctyunapi.cn", result)
}
