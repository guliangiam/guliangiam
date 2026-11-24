package pool

import (
	"bytes"
	"sync"
)

var BufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 4096))
	},
}

func GetBuffer() *bytes.Buffer {
	return BufferPool.Get().(*bytes.Buffer)
}

func PutBuffer(b *bytes.Buffer) {
	b.Reset()
	BufferPool.Put(b)
}
