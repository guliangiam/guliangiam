package pool

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBuffer(t *testing.T) {
	b := GetBuffer()
	assert.NotNil(t, b)
	assert.Equal(t, 0, b.Len(), "new buffer should be empty")
	assert.True(t, b.Cap() > 0, "buffer should have capacity")
}

func TestPutBuffer(t *testing.T) {
	b := GetBuffer()
	b.WriteString("test data")
	assert.Equal(t, 9, b.Len())

	PutBuffer(b)
	// After PutBuffer, the buffer is reset and returned to pool
	// Getting a new buffer should give us a clean one
	b2 := GetBuffer()
	assert.Equal(t, 0, b2.Len(), "recycled buffer should be empty")
	PutBuffer(b2)
}

func TestBufferPool_RoundTrip(t *testing.T) {
	// Write data, return to pool, get again - should be clean
	b := GetBuffer()
	b.WriteString("hello world")
	PutBuffer(b)

	b2 := GetBuffer()
	b2.WriteString("new data")
	assert.Equal(t, "new data", b2.String())
	PutBuffer(b2)
}

func TestBufferPool_MultipleBuffers(t *testing.T) {
	b1 := GetBuffer()
	b2 := GetBuffer()

	b1.WriteString("data1")
	b2.WriteString("data2")

	assert.Equal(t, "data1", b1.String())
	assert.Equal(t, "data2", b2.String())

	PutBuffer(b1)
	PutBuffer(b2)
}

func TestGetBuffer_Type(t *testing.T) {
	b := GetBuffer()
	if b == nil {
		t.Fatal("GetBuffer() returned nil")
	}
	PutBuffer(b)
}
