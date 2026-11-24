package workpool

import (
	"context"
	"testing"
	"time"
)

// BE-E-19 回归：单个任务 panic 不得杀死 worker goroutine。

func TestPool_TaskPanicDoesNotKillWorker(t *testing.T) {
	p := NewPool(1, 4)
	defer p.Shutdown()

	if err := p.Submit(context.Background(), func() { panic("bad task") }); err != nil {
		t.Fatalf("submit panic task: %v", err)
	}

	done := make(chan struct{})
	if err := p.Submit(context.Background(), func() { close(done) }); err != nil {
		t.Fatalf("submit follow-up task: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker died after task panic; follow-up task never ran")
	}
}
