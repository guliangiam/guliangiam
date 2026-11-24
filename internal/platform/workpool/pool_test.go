package workpool

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPool_SubmitAndExecute(t *testing.T) {
	p := NewPool(2, 10)
	defer p.Shutdown()

	var executed atomic.Int32
	err := p.Submit(context.Background(), func() {
		executed.Add(1)
	})
	assert.NoError(t, err)

	// Wait for task to complete
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), executed.Load())
}

func TestPool_MultipleTasks(t *testing.T) {
	p := NewPool(4, 20)
	defer p.Shutdown()

	var executed atomic.Int32
	for i := 0; i < 10; i++ {
		err := p.Submit(context.Background(), func() {
			executed.Add(1)
		})
		assert.NoError(t, err)
	}

	// Wait for all tasks
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(10), executed.Load())
}

func TestPool_QueueFull(t *testing.T) {
	// Small queue to test overflow
	p := NewPool(1, 2)
	defer p.Shutdown()

	// Fill the queue with blocking tasks
	blockCh := make(chan struct{})
	for i := 0; i < 2; i++ {
		err := p.Submit(context.Background(), func() {
			<-blockCh // block worker
		})
		assert.NoError(t, err)
	}

	// Next submit should fail (queue full)
	err := p.Submit(context.Background(), func() {})
	assert.Equal(t, ErrPoolFull, err)

	close(blockCh) // unblock workers
}

func TestPool_CancelledContext(t *testing.T) {
	p := NewPool(2, 10)
	defer p.Shutdown()

	var executed atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := p.Submit(ctx, func() {
		executed.Add(1)
	})
	assert.NoError(t, err) // submit succeeds but task is skipped

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(0), executed.Load(), "task should be skipped due to cancelled context")
}

func TestPool_Shutdown(t *testing.T) {
	p := NewPool(2, 10)
	p.Shutdown() // should not block indefinitely

	// Submit after shutdown should fail
	err := p.Submit(context.Background(), func() {})
	assert.Equal(t, ErrPoolFull, err)
}

func TestPool_DoubleShutdown(t *testing.T) {
	p := NewPool(1, 1)
	p.Shutdown()
	p.Shutdown() // should not panic
}

func TestPool_ShutdownWithTimeout(t *testing.T) {
	p := NewPool(1, 5)

	// Submit a slow task
	if err := p.Submit(context.Background(), func() {
		time.Sleep(50 * time.Millisecond)
	}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	p.ShutdownWithTimeout(1 * time.Second) // should complete within timeout
}
